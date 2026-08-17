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
		temp     = fs.Float64("temperature", -1, "temperature; negative leaves it unset")
		topP     = fs.Float64("top-p", -1, "top_p; negative leaves it unset")
		topK     = fs.Int("top-k", -1, "top_k; negative leaves it unset")
		presPen  = fs.Float64("presence-penalty", -1, "presence_penalty; negative leaves it unset")
		timeout  = fs.Duration("timeout", 15*time.Minute, "per-request timeout")
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
	var sampling eval.Sampling
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
			res, err := client.Run(ctx, task, sampling, think)
			if err != nil {
				// A transport failure is recorded and the suite continues: losing hours
				// of sweep to one dropped connection would be worse than a gap.
				_, _ = fmt.Fprintf(stderr, "eval: %s: %v\n", task.ID, err)
				failures++
				continue
			}
			if *results != "" {
				row := eval.NewRow(*config, rep, *thinking, sampling, props, task.Kind, res)
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
