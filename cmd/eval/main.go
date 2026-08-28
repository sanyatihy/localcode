// Command eval scores fixed tasks against a local OpenAI-compatible endpoint and
// appends one row per observation to a results file.
//
// Tier 1 only: single-turn tasks with deterministic checks, which is what the
// serving sweeps need and what runs in seconds rather than minutes.
//
// Exit codes are the contract, so a sweep can branch without parsing output:
//
//	0  every task passed
//	1  the run completed and at least one task failed
//	2  the run could not be carried out (bad flags, unreadable fixture, no server)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sanyatihy/localcode/internal/build"
	"github.com/sanyatihy/localcode/internal/eval"
)

// errTasksFailed distinguishes "the measurement ran and the answer is no" from
// "the measurement could not be taken" — the two exit codes a sweep branches on.
var errTasksFailed = errors.New("one or more tasks failed")

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, errTasksFailed) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "eval: %v\n", err)
		os.Exit(2)
	}
}

func run(args []string, stdout, stderr *os.File) error {
	fs := flag.NewFlagSet("eval", flag.ContinueOnError)
	version := fs.Bool("version", false, "print which build this is, and exit")
	fs.SetOutput(stderr)
	var (
		endpoint = fs.String("endpoint", "http://127.0.0.1:8081", "OpenAI-compatible endpoint")
		machine  = fs.String("machine", "config/machine.json", "machine properties: the headroom a sweep needs")
		force    = fs.Bool("force", false, "start even when the preflight refuses, and mark every row forced")
		taskPath = fs.String("task", "", "path to a single task fixture")
		tasksDir = fs.String("tasks", "", "directory of task fixtures to run as a suite")
		repeats  = fs.Int("n", 1, "passes over the suite")
		config   = fs.String("config", "unlabelled", "label for the serving config under test")
		results  = fs.String("results", "", "append a JSONL row here; empty writes none")
		// Streaming is what separates decode from prefill, and a session is what makes
		// two configs' decode rates divisible: rows sharing one were measured back to
		// back on one machine state. An unpaired before-and-after measures host drift.
		stream   = fs.Bool("stream", false, "stream the reply so decode is measured apart from prefill")
		session  = fs.String("session", "", "label pairing this run with the baseline it is read against")
		fidelity = fs.Bool("fidelity", false, "after the suite, hash fixed greedy probes so the pair can be checked for losslessness")
		probeSet = fs.String("fidelity-probes", "short", "short|prefill: which fixed probes to hash. prefill uses prompts long enough that a physical batch size divides them differently")
		thinking = fs.String("thinking", "", "enable_thinking: on, off, or empty for the template default")
		// Passed through rather than checked against a list: the next model's vocabulary
		// differs and the server rejects what it does not know. What is guaranteed here
		// is that whatever was sent appears on every row.
		effort = fs.String("reasoning-effort", "", "reasoning_effort passed to the server verbatim; empty leaves the model default")
		// The values come from the model profile rather than constants here: the pair is a
		// property of the model, and getting it wrong silently voided 114 rows.
		profile     = fs.String("sampling-profile", "", "thinking|nonthinking: apply the model's recommended sampling for that mode")
		profilePath = fs.String("model-profile", "config/profiles/qwen3.8.json", "model profile: thinking mechanism, per-mode sampling, reasoning extraction")
		temp        = fs.Float64("temperature", -1, "temperature; negative leaves it unset")
		topP        = fs.Float64("top-p", -1, "top_p; negative leaves it unset")
		topK        = fs.Int("top-k", -1, "top_k; negative leaves it unset")
		presPen     = fs.Float64("presence-penalty", -1, "presence_penalty; negative leaves it unset")
		timeout     = fs.Duration("timeout", 15*time.Minute, "per-request timeout")
		// The dialect, not the backend. `messages` is the Anthropic path llama-server
		// converts internally and the one a Claude Code session takes, so a number taken
		// on the chat path says nothing about what the editor flow gets.
		api = fs.String("api", eval.APIChat, "request dialect: chat or messages")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Before anything else is read: a build that cannot say what it is has nothing
	// to say about a machine either.
	if *version {
		_, _ = fmt.Fprintln(stdout, build.Version("eval"))
		return nil
	}

	if *taskPath != "" && *tasksDir != "" {
		return errors.New("give exactly one of -task or -tasks")
	}
	// A fidelity run with no fixtures is a whole run. The probes are an instrument, and a
	// change to how a prefill is split has to be checked with them before anything
	// expensive is scored on it — so asking for the instrument alone is not a mistake.
	if *taskPath == "" && *tasksDir == "" && !*fidelity {
		return errors.New("give exactly one of -task or -tasks, or -fidelity on its own")
	}
	var paths []string
	if *taskPath != "" {
		paths = []string{*taskPath}
	}
	if *tasksDir != "" {
		var err error
		if paths, err = eval.DiscoverTasks(*tasksDir); err != nil {
			return err
		}
		if len(paths) == 0 {
			return fmt.Errorf("no fixtures under %s", *tasksDir)
		}
	}

	if *api != eval.APIChat && *api != eval.APIMessages {
		return fmt.Errorf("-api %q: want %s or %s", *api, eval.APIChat, eval.APIMessages)
	}

	think, err := parseThinking(*thinking)
	if err != nil {
		return err
	}

	// The toggle carries its own sampling, so comparing modes at one fixed setting
	// measures the pair and not the toggle — 114 rows were collected that way. Refused
	// rather than defaulted: there is no neutral setting to fall back on.
	if *thinking != "" && *profile == "" && *temp < 0 && *topP < 0 && *topK < 0 && *presPen < 0 {
		return errors.New("-thinking was set with no sampling: pass -sampling-profile thinking|nonthinking, " +
			"or set the sampling flags explicitly. Comparing modes at one fixed sampling measures the pair, not the toggle")
	}

	var sampling eval.Sampling
	prof, err := eval.LoadProfile(*profilePath)
	if err != nil {
		return fmt.Errorf("model profile: %w", err)
	}
	if *profile != "" {
		s, err := prof.SamplingFor(*profile)
		if err != nil {
			return err
		}
		sampling = s
	}
	if *temp >= 0 {
		sampling.Temperature = temp
	}
	if *topP >= 0 {
		sampling.TopP = topP
	}
	if *topK >= 0 {
		sampling.TopK = topK
	}
	if *presPen >= 0 {
		sampling.PresencePenalty = presPen
	}

	// Asked before the first task, because a sweep that pages measures the pager and the
	// rows say so only once the machine time is spent.
	machineCfg, err := eval.LoadMachine(*machine)
	if err != nil {
		return err
	}
	pre := eval.Check(eval.Sample(), machineCfg.MinHeadroomGB)
	if why := pre.Refuse(); why != "" {
		if !*force {
			return fmt.Errorf("the machine cannot carry this sweep: %s", why)
		}
		_, _ = fmt.Fprintf(stderr, "eval: starting anyway under -force: %s\n", why)
	}

	ctx := context.Background()
	client := eval.NewClient(*endpoint, *timeout)
	client.API = *api
	client.Stream = *stream

	// A backend that cannot introspect is still scoreable — MLX serves completions without
	// llama.cpp's /props. What is lost is the guard on the label a human typed, so the run
	// says so rather than recording a confident zero that reads as "0 context".
	props, err := client.Props(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "eval: %s exposes no served config (%v); "+
			"rows will record it as unavailable and the served-config guard is off\n", *endpoint, err)
		props = eval.ServerProps{}
	}

	// Sequential: the server runs one slot, so concurrent requests would measure the queue.
	failures := 0
	for rep := range *repeats {
		for _, p := range paths {
			task, err := eval.LoadTask(p)
			if err != nil {
				return err
			}
			res, err := client.Run(ctx, task, sampling, think, *effort, prof)
			if err != nil {
				// The suite continues: losing hours of sweep to one dropped connection
				// is worse than a gap in the rows.
				_, _ = fmt.Fprintf(stderr, "eval: %s: %v\n", task.ID, err)
				failures++
				continue
			}
			if *results != "" {
				row := eval.NewRow(*config, rep, *thinking, *effort, sampling, props, task.Kind, res)
				row.Forced = *force
				row.Session = *session
				if err := eval.AppendRow(*results, row); err != nil {
					return fmt.Errorf("cannot append result: %w", err)
				}
			}
			status := "PASS"
			if !res.Passed() {
				status = "FAIL"
				failures++
			}
			_, _ = fmt.Fprintf(stdout, "%-4s [%d] %-24s %s %s\n", status, rep, res.TaskID, res.Outcome, res.Detail)
			_, _ = fmt.Fprintf(stdout, "          ctx %d | prompt %d tok (%d cached) @ %.1f tok/s | gen %d tok @ %.1f tok/s | reasoning %d chars | wall %.1fs\n",
				props.NCtx, res.PromptTokens, res.CachedTokens, res.PromptPerSecond,
				res.CompletionTokens, res.GenPerSecond, res.ReasoningChars, res.WallSeconds)
		}
	}
	// After the suite rather than before it: the probes are an instrument, and running
	// them first would warm caches the first scored task should pay for itself.
	if *fidelity {
		probes, err := eval.ProbeSet(*probeSet)
		if err != nil {
			return err
		}
		res, err := eval.Fidelity(ctx, client, prof, 256, probes)
		if err != nil {
			return fmt.Errorf("fidelity probe: %w", err)
		}
		_, _ = fmt.Fprintf(stdout, "fidelity %s %s\n", *probeSet, res.Hash[:16])
		if *results != "" {
			row := eval.NewRow(*config, 0, "off", *effort, sampling, props, "fidelity",
				eval.Result{TaskID: "fidelity-probe", Outcome: eval.Pass})
			row.Session, row.FidelityHash, row.Forced = *session, res.Hash, *force
			row.FidelityProbes = *probeSet
			if err := eval.AppendRow(*results, row); err != nil {
				return fmt.Errorf("cannot append fidelity row: %w", err)
			}
		}
	}

	if failures > 0 {
		return fmt.Errorf("%w: %d", errTasksFailed, failures)
	}
	return nil
}

func parseThinking(s string) (*bool, error) {
	switch s {
	case "on":
		v := true
		return &v, nil
	case "off":
		v := false
		return &v, nil
	case "":
		return nil, nil // leave the model's own template default alone
	default:
		return nil, fmt.Errorf("-thinking must be on, off, or empty, got %q", s)
	}
}
