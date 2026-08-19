// Command tier2 drives a coding harness through a fixture in a scratch checkout and
// scores it by running tests the harness never saw.
//
// Tier 1 measures a single request; this measures a whole agent loop, which is the only
// way to see multi-turn behaviour — how many turns a harness spends, whether it recovers
// from a bad tool call, how much context it burns getting to the same answer.
//
// Exit codes are the contract, so a comparison script can branch without parsing output:
//
//	0  every harness passed the task
//	1  the run completed and at least one harness failed it
//	2  the run could not be carried out (bad flags, unreadable fixture, broken adapter)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sanyatihy/localcode/internal/eval"
	"github.com/sanyatihy/localcode/internal/harness"
)

var errTaskFailed = errors.New("one or more harnesses failed the task")

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, errTaskFailed) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "tier2: %v\n", err)
		os.Exit(2)
	}
}

// config is resolved once, here, and validated before anything runs. Flags that name a
// file are checked for readability by the adapters at drive time rather than here, so the
// error names the harness that needed them.
type config struct {
	drivers     []string
	fixture     string
	source      string
	testFile    string
	answerName  string
	instruction string
	piExtension string
	ocConfig    string
	ccEnv       string
	model       string
	results     string
	label       string
	keep        bool
}

func run(args []string, stdout, stderr *os.File) error {
	fs := flag.NewFlagSet("tier2", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		drivers = fs.String("drivers", "pi,opencode", "comma-separated: pi, opencode, hermes, claude-code")
		fixture = fs.String("fixture", "tasks/patch-nil-check", "fixture directory")
		source  = fs.String("source", "broken.go.txt", "file in the fixture the harness must fix")
		test    = fs.String("test", "verify_test.go.txt", "unseen test staged beside the answer")
		answer  = fs.String("answer-name", "session.go", "name the source takes in the scratch module")
		instr   = fs.String("instruction", "", "what to tell the harness (required)")
		piExt   = fs.String("pi-extension", "harness/pi/local-provider.js", "pi provider extension")
		ocCfg   = fs.String("opencode-config", "harness/opencode/opencode.json", "opencode provider config")
		ccEnv   = fs.String("claude-code-env", "harness/claude-code/claude-code.env", "claude code environment file")
		model   = fs.String("model", "bartowski/Qwen3.8-27B-GGUF:Q4_K_M", "served model id")
		results = fs.String("results", "", "append a JSONL row here; empty writes none")
		label   = fs.String("label", "unlabelled", "serving config label recorded with each row")
		keep    = fs.Bool("keep", false, "leave the scratch checkout in place and print its path")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg := config{
		drivers: splitNonEmpty(*drivers), fixture: *fixture, source: *source,
		testFile: *test, answerName: *answer, instruction: *instr,
		piExtension: *piExt, ocConfig: *ocCfg, ccEnv: *ccEnv, model: *model,
		results: *results, label: *label, keep: *keep,
	}
	if err := (&cfg).validate(); err != nil {
		return err
	}

	ds, err := buildDrivers(cfg)
	if err != nil {
		return err
	}

	task := eval.Tier2Task{
		// Base name, not the resolved path: the id is a grouping key in results and
		// must not change because the repo moved.
		ID:          filepath.Base(cfg.fixture),
		Dir:         cfg.fixture,
		Source:      cfg.source,
		TestFile:    cfg.testFile,
		AnswerName:  cfg.answerName,
		Instruction: cfg.instruction,
	}

	ctx := context.Background()
	failures := 0
	// Sequential: the server runs one slot, so concurrent harnesses would queue and
	// every duration would measure the queue rather than the harness.
	for _, d := range ds {
		res, work, err := eval.RunTier2(ctx, d, task, cfg.keep)
		if err != nil {
			// A staging or fixture problem is not a result about the harness.
			return fmt.Errorf("%s: %w", d.Name(), err)
		}
		status := "PASS"
		if !res.Passed() {
			status = "FAIL"
			failures++
		}
		_, _ = fmt.Fprintf(stdout, "%-4s %-10s %-24s %s %s\n",
			status, d.Name(), res.TaskID, res.Outcome, res.Detail)
		_, _ = fmt.Fprintf(stdout, "     %.1fs\n", res.WallSeconds)
		if cfg.keep {
			_, _ = fmt.Fprintf(stdout, "     scratch: %s\n", work)
		}
		if cfg.results != "" {
			// Empty effort, and honestly so: tier-2 drives an external harness that
			// builds its own requests, so what it asked for is the harness's business
			// and not something this process can claim to have set.
			row := eval.NewRow(cfg.label, 0, "", "", eval.Sampling{}, eval.ServerProps{}, "tier2", res)
			row.Detail = strings.TrimSpace(d.Name() + " " + row.Detail)
			if err := eval.AppendRow(cfg.results, row); err != nil {
				return fmt.Errorf("append result: %w", err)
			}
		}
	}
	if failures > 0 {
		return fmt.Errorf("%w: %d of %d", errTaskFailed, failures, len(ds))
	}
	return nil
}

// validate checks the run can be carried out and resolves every path to absolute.
//
// The absolute-path step is load-bearing rather than tidiness: adapters run their CLI
// with the working directory set to the scratch checkout, so a relative config path
// resolves against that temporary directory instead of the repo. Left relative, pi looks
// for its extension inside /tmp and fails with a message about the extension rather than
// about the path.
func (c *config) validate() error {
	if c.instruction == "" {
		return errors.New("-instruction is required")
	}
	if len(c.drivers) == 0 {
		return errors.New("-drivers named none")
	}
	for _, p := range []*string{&c.fixture, &c.piExtension, &c.ocConfig, &c.ccEnv} {
		abs, err := filepath.Abs(*p)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", *p, err)
		}
		*p = abs
	}
	if _, err := os.Stat(c.fixture); err != nil {
		return fmt.Errorf("fixture directory: %w", err)
	}
	return nil
}

// buildDrivers returns concrete adapters behind the interface the caller consumes.
func buildDrivers(c config) ([]eval.Driver, error) {
	var ds []eval.Driver
	for _, name := range c.drivers {
		switch name {
		case "pi":
			ds = append(ds, harness.NewPi(c.piExtension, "local", c.model))
		case "opencode":
			ds = append(ds, harness.NewOpenCode(c.ocConfig, "local/"+c.model))
		case "claude-code":
			ds = append(ds, harness.NewClaudeCode(c.ccEnv))
		case "hermes":
			// Hermes reads a global config and refuses anything under 64k context, so
			// it takes no per-run parameters here — see internal/harness/hermes.go.
			ds = append(ds, harness.NewHermes())
		default:
			return nil, fmt.Errorf("unknown driver %q", name)
		}
	}
	return ds, nil
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
