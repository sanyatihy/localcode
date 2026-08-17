// Command eval scores fixed tasks against a local OpenAI-compatible endpoint.
//
// Tier 1 only for now: single-turn tasks with deterministic checks, which is what
// the serving sweeps need and what runs in seconds rather than minutes.
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
		taskPath = flag.String("task", "", "path to a task fixture (required)")
		thinking = flag.String("thinking", "", "enable_thinking: on, off, or empty for the template default")
		temp     = flag.Float64("temperature", -1, "temperature; negative leaves it unset")
		timeout  = flag.Duration("timeout", 10*time.Minute, "per-request timeout")
	)
	flag.Parse()

	if *taskPath == "" {
		fmt.Fprintln(os.Stderr, "eval: -task is required")
		os.Exit(2)
	}
	task, err := eval.LoadTask(*taskPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval: %v\n", err)
		os.Exit(2)
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
		fmt.Fprintf(os.Stderr, "eval: -thinking must be on, off, or empty\n")
		os.Exit(2)
	}

	var sampling eval.Sampling
	if *temp >= 0 {
		sampling.Temperature = temp
	}

	client := eval.NewClient(*endpoint, *timeout)
	res, err := client.Run(context.Background(), task, sampling, think)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval: %s: %v\n", task.ID, err)
		os.Exit(2)
	}

	status := "PASS"
	if !res.Passed() {
		status = "FAIL"
	}
	fmt.Printf("%-4s %-24s %s %s\n", status, res.TaskID, res.Outcome, res.Detail)
	fmt.Printf("     prompt %d tok (%d cached) @ %.1f tok/s | gen %d tok @ %.1f tok/s | reasoning %d chars | wall %.1fs\n",
		res.PromptTokens, res.CachedTokens, res.PromptPerSecond,
		res.CompletionTokens, res.GenPerSecond, res.ReasoningChars, res.WallSeconds)

	// Exit code is the contract: a sweep script must be able to tell pass from fail
	// without parsing output. 1 is a failed task, 2 is a broken run.
	if !res.Passed() {
		os.Exit(1)
	}
}
