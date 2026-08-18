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
	fs.SetOutput(stderr)
	var (
		endpoint = fs.String("endpoint", "http://127.0.0.1:8080", "OpenAI-compatible endpoint")
		taskPath = fs.String("task", "", "path to a single task fixture")
		tasksDir = fs.String("tasks", "", "directory of task fixtures to run as a suite")
		repeats  = fs.Int("n", 1, "passes over the suite")
		config   = fs.String("config", "unlabelled", "label for the serving config under test")
		results  = fs.String("results", "", "append a JSONL row here; empty writes none")
		thinking = fs.String("thinking", "", "enable_thinking: on, off, or empty for the template default")
		// Passed through rather than validated against a list. Qwen3.8 takes
		// low/medium/xhigh; the next model will take something else, and a harness that
		// hardcodes one vendor's vocabulary has to be edited before it can measure
		// anything new. The server rejects what it does not know. What this process
		// guarantees instead is that whatever was sent is on every row.
		effort = fs.String("reasoning-effort", "", "reasoning_effort passed to the server verbatim; empty leaves the model default")
		// The mode's recommended sampling, as one flag. The pair is documented in
		// docs/VISION.md and getting it wrong silently is what voided 114 rows.
		profile = fs.String("sampling-profile", "", "thinking|nonthinking: apply the model's recommended sampling for that mode")
		temp    = fs.Float64("temperature", -1, "temperature; negative leaves it unset")
		topP    = fs.Float64("top-p", -1, "top_p; negative leaves it unset")
		topK    = fs.Int("top-k", -1, "top_k; negative leaves it unset")
		presPen = fs.Float64("presence-penalty", -1, "presence_penalty; negative leaves it unset")
		timeout = fs.Duration("timeout", 15*time.Minute, "per-request timeout")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if (*taskPath == "") == (*tasksDir == "") {
		return errors.New("give exactly one of -task or -tasks")
	}
	paths := []string{*taskPath}
	if *tasksDir != "" {
		var err error
		if paths, err = eval.DiscoverTasks(*tasksDir); err != nil {
			return err
		}
		if len(paths) == 0 {
			return fmt.Errorf("no fixtures under %s", *tasksDir)
		}
	}

	think, err := parseThinking(*thinking)
	if err != nil {
		return err
	}

	// Negative means "not set", so the server's own default applies. Zero is a real
	// value a sweep may want, and must stay distinguishable from silence.
	// The toggle carries its own sampling, so comparing modes at one fixed setting
	// measures the pair rather than the toggle — the vision calls such a result void,
	// and 114 rows were collected that way before anyone noticed. Setting the mode
	// without saying which sampling goes with it is refused rather than defaulted:
	// the whole point is that there is no neutral setting to fall back on.
	if *thinking != "" && *profile == "" && *temp < 0 && *topP < 0 && *topK < 0 && *presPen < 0 {
		return errors.New("-thinking was set with no sampling: pass -sampling-profile thinking|nonthinking, " +
			"or set the sampling flags explicitly. Comparing modes at one fixed sampling measures the pair, not the toggle")
	}

	var sampling eval.Sampling
	switch *profile {
	case "":
	case "thinking":
		t, p, k := 1.0, 0.95, 20
		sampling.Temperature, sampling.TopP, sampling.TopK = &t, &p, &k
	case "nonthinking":
		t, p, k, pp := 0.7, 0.80, 20, 1.5
		sampling.Temperature, sampling.TopP, sampling.TopK, sampling.PresencePenalty = &t, &p, &k, &pp
	default:
		return fmt.Errorf("-sampling-profile must be thinking or nonthinking, got %q", *profile)
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

	ctx := context.Background()
	client := eval.NewClient(*endpoint, *timeout)

	props, err := client.Props(ctx)
	if err != nil {
		return fmt.Errorf("cannot read server props from %s: %w", *endpoint, err)
	}

	// Sequential on purpose: the server runs one slot, so concurrent requests would
	// queue and every timing would measure the queue instead of the model.
	failures := 0
	for rep := range *repeats {
		for _, p := range paths {
			task, err := eval.LoadTask(p)
			if err != nil {
				return err
			}
			res, err := client.Run(ctx, task, sampling, think, *effort)
			if err != nil {
				// A transport failure is recorded and the suite continues: losing hours
				// of sweep to one dropped connection would be worse than a gap.
				_, _ = fmt.Fprintf(stderr, "eval: %s: %v\n", task.ID, err)
				failures++
				continue
			}
			if *results != "" {
				row := eval.NewRow(*config, rep, *thinking, *effort, sampling, props, task.Kind, res)
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
