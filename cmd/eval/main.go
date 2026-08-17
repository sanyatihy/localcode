// Command eval scores fixed tasks against a local OpenAI-compatible endpoint and
// appends one row per observation to a results file.
//
// Tier 1 only: single-turn tasks with deterministic checks, which is what the
// serving sweeps need and what runs in seconds rather than minutes.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sanyatihy/localcode/internal/eval"
)

func main() {
	var (
		endpoint = flag.String("endpoint", "http://127.0.0.1:8080", "OpenAI-compatible endpoint")
		taskPath = flag.String("task", "", "path to a single task fixture")
		tasksDir = flag.String("tasks", "", "directory of task fixtures to run as a suite")
		repeats  = flag.Int("n", 1, "passes over the suite")
		config   = flag.String("config", "unlabelled", "label for the serving config under test")
		results  = flag.String("results", "", "append a JSONL row here; empty writes none")
		thinking = flag.String("thinking", "", "enable_thinking: on, off, or empty for the template default")
		temp     = flag.Float64("temperature", -1, "temperature; negative leaves it unset")
		topP     = flag.Float64("top-p", -1, "top_p; negative leaves it unset")
		topK     = flag.Int("top-k", -1, "top_k; negative leaves it unset")
		presPen  = flag.Float64("presence-penalty", -1, "presence_penalty; negative leaves it unset")
		timeout  = flag.Duration("timeout", 15*time.Minute, "per-request timeout")
	)
	flag.Parse()

	if (*taskPath == "") == (*tasksDir == "") {
		fmt.Fprintln(os.Stderr, "eval: give exactly one of -task or -tasks")
		os.Exit(2)
	}

	var paths []string
	if *taskPath != "" {
		paths = []string{*taskPath}
	} else {
		var err error
		if paths, err = eval.DiscoverTasks(*tasksDir); err != nil {
			fmt.Fprintf(os.Stderr, "eval: %v\n", err)
			os.Exit(2)
		}
		if len(paths) == 0 {
			fmt.Fprintf(os.Stderr, "eval: no fixtures under %s\n", *tasksDir)
			os.Exit(2)
		}
	}

	var think *bool
	switch *thinking {
	case "on":
		v := true
		think = &v
	case "off":
		v := false
		think = &v
	case "":
	default:
		fmt.Fprintln(os.Stderr, "eval: -thinking must be on, off, or empty")
		os.Exit(2)
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
		fmt.Fprintf(os.Stderr, "eval: cannot read server props: %v\n", err)
		os.Exit(2)
	}

	// Sequential on purpose: the server runs one slot, so concurrent requests would
	// queue and every timing would measure the queue instead of the model.
	failures := 0
	for rep := 0; rep < *repeats; rep++ {
		for _, p := range paths {
			task, err := eval.LoadTask(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "eval: %v\n", err)
				os.Exit(2)
			}
			res, err := client.Run(ctx, task, sampling, think)
			if err != nil {
				// A transport failure is recorded and the suite continues: losing four
				// hours of sweep to one dropped connection would be worse than a gap.
				fmt.Fprintf(os.Stderr, "eval: %s: %v\n", task.ID, err)
				failures++
				continue
			}
			if *results != "" {
				row := eval.NewRow(*config, rep, *thinking, sampling, props, task.Kind, res)
				if err := eval.AppendRow(*results, row); err != nil {
					fmt.Fprintf(os.Stderr, "eval: cannot append result: %v\n", err)
					os.Exit(2)
				}
			}
			status := "PASS"
			if !res.Passed() {
				status = "FAIL"
				failures++
			}
			fmt.Printf("%-4s [%d] %-24s %s %s\n", status, rep, res.TaskID, res.Outcome, res.Detail)
			fmt.Printf("          ctx %d | prompt %d tok (%d cached) @ %.1f tok/s | gen %d tok @ %.1f tok/s | reasoning %d chars | wall %.1fs\n",
				props.NCtx, res.PromptTokens, res.CachedTokens, res.PromptPerSecond,
				res.CompletionTokens, res.GenPerSecond, res.ReasoningChars, res.WallSeconds)
		}
	}

	// Exit code is the contract: a sweep must tell pass from fail without parsing
	// output. 1 means at least one task failed, 2 means the run itself broke.
	if failures > 0 {
		os.Exit(1)
	}
}
