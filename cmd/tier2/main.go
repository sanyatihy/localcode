// Command tier2 drives a coding harness through one fixture or a whole suite of them, each
// in a scratch checkout, and scores it by running tests the harness never saw.
//
// What the harness is told comes from the fixture, never from a flag: the bug statement is
// the one a tier-1 request would carry, minus the source it inlines, so a row's task id is
// enough to recover the instruction that produced it.
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
//
// A harness the desk profile excludes is none of those: it never ran, so it is reported
// and recorded as inadmissible and leaves the exit code alone.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sanyatihy/localcode/internal/eval"
	"github.com/sanyatihy/localcode/internal/harness"
)

var errTaskFailed = errors.New("one or more harnesses failed the task")

// propsTimeout bounds the one question this command asks the server directly. Short
// because it is a local endpoint answering from memory, and a run should not spend a
// harness timeout discovering the server is not there.
const propsTimeout = 15 * time.Second

// turnPollInterval is how often the server's slot is asked what it is working on. Short
// enough that no turn fits inside it: at the contexts this comparison runs, a turn spends
// seconds ingesting before it generates anything.
const turnPollInterval = 150 * time.Millisecond

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
	fixture     string // one fixture directory
	fixtures    string // directory of them, scanned for what tier 2 can drive
	desk        eval.DeskProfile
	endpoint    string
	piExtension string
	ocConfig    string
	ccEnv       string
	hermesCfg   string
	model       string
	results     string
	label       string
	repeats     int
	sandbox     string // sandbox profile applied to the harness; empty runs it online
	forced      bool
	budget      time.Duration
	keep        bool
}

func run(args []string, stdout, stderr *os.File) error {
	fs := flag.NewFlagSet("tier2", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		drivers  = fs.String("drivers", "pi,opencode", "comma-separated: pi, opencode, hermes, claude-code")
		fixture  = fs.String("fixture", "", "one fixture directory")
		fixtures = fs.String("fixtures", "", "directory of fixtures; every one tier 2 can drive is run")
		profile  = fs.String("profile", "attended", "desk profile the run is scored under: attended, unattended")
		endpoint = fs.String("endpoint", "http://127.0.0.1:8081", "endpoint the harnesses are pointed at, asked what it serves")
		machine  = fs.String("machine", "config/machine.json", "machine properties: the headroom a sweep needs")
		force    = fs.Bool("force", false, "start even when the preflight refuses, and mark every row forced")
		piExt    = fs.String("pi-extension", "harness/pi/local-provider.js", "pi provider extension")
		ocCfg    = fs.String("opencode-config", "harness/opencode/opencode.json", "opencode provider config")
		ccEnv    = fs.String("claude-code-env", "harness/claude-code/claude-code.env", "claude code environment file")
		hermes   = fs.String("hermes-config", "harness/hermes/config.yaml.reference", "hermes config, seeded into each run's own home")
		model    = fs.String("model", "bartowski/Qwen3.8-27B-GGUF:Q4_K_M", "served model id")
		results  = fs.String("results", "", "append a JSONL row here; empty writes none")
		label    = fs.String("label", "unlabelled", "serving config label recorded with each row")
		repeats  = fs.Int("n", 1, "passes over the whole set; a pass is every harness over every fixture")
		budget   = fs.Duration("budget", eval.DefaultBudget, "how long one harness may spend on one fixture before it is over budget")
		offline  = fs.Bool("offline", false, "run each harness with no network but the loopback the model is on")
		sandbox  = fs.String("sandbox-profile", "harness/offline.sb", "sandbox profile -offline applies")
		keep     = fs.Bool("keep", false, "leave the scratch checkout in place and print its path")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	desk, err := eval.LookupDeskProfile(*profile)
	if err != nil {
		return err
	}
	cfg := config{
		forced:  *force,
		drivers: splitNonEmpty(*drivers), fixture: *fixture, fixtures: *fixtures,
		desk: desk, endpoint: *endpoint,
		piExtension: *piExt, ocConfig: *ocCfg, ccEnv: *ccEnv, hermesCfg: *hermes, model: *model,
		results: *results, label: *label, repeats: *repeats, budget: *budget, keep: *keep,
	}
	if *offline {
		cfg.sandbox = *sandbox
	}
	if err := (&cfg).validate(); err != nil {
		return err
	}

	ds, err := buildDrivers(cfg)
	if err != nil {
		return err
	}

	// Asked before the first task: a sweep that pages measures the pager, and tier 2 spends
	// hours rather than minutes finding that out.
	machineCfg, err := eval.LoadMachine(*machine)
	if err != nil {
		return err
	}
	if why := eval.Check(eval.Sample(), machineCfg.MinHeadroomGB).Refuse(); why != "" {
		if !*force {
			return fmt.Errorf("the machine cannot carry this sweep: %s", why)
		}
		_, _ = fmt.Fprintf(stderr, "tier2: starting anyway under -force: %s\n", why)
	}

	tasks, err := loadTasks(cfg)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Asked rather than told: the profile a run declares is a human's claim, and a harness
	// driven below its floor fails in a way that reads as the model answering badly. A
	// backend that cannot be asked is still scoreable, so the guard switches off loudly.
	client := eval.NewClient(cfg.endpoint, propsTimeout)
	props, err := client.Props(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tier2: %s does not say what it serves (%v); rows record it "+
			"as unavailable and a harness's context floor is checked against the profile alone\n",
			cfg.endpoint, err)
		props = eval.ServerProps{}
	}
	// Serving above the profile's ceiling makes every row of this run false: the label
	// would say a machine somebody could use, and 0014 measured that context as one they
	// could not. A restart is the fix, so this stops the run rather than marking rows.
	if props.Available && props.NCtx > cfg.desk.Ceiling {
		return fmt.Errorf("server is serving %d, above the %s ceiling of %d: restart it lower, "+
			"or score this run as unattended", props.NCtx, cfg.desk.Name, cfg.desk.Ceiling)
	}

	// Sampled once here so a server without --metrics is reported before the sweep
	// rather than as a column of zeroes afterwards.
	if _, err := client.Metrics(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "tier2: %s counts no tokens (%v); rows will carry none. "+
			"Serve with METRICS=1 to record what each run cost\n", cfg.endpoint, err)
	}

	failures, runs := 0, 0
	// Sequential and driver-major: the server runs one slot, so concurrent harnesses would
	// measure the queue. Repeats are the outer loop — three runs of one fixture back to
	// back would leave the later two reading a prefix the first paid for.
	for rep := range cfg.repeats {
		for _, d := range ds {
			for _, task := range tasks {
				failed, err := runOne(ctx, stdout, client, d, task, props, cfg, rep)
				if err != nil {
					return err
				}
				runs++
				if failed {
					failures++
				}
			}
		}
	}
	if failures > 0 {
		return fmt.Errorf("%w: %d of %d", errTaskFailed, failures, runs)
	}
	return nil
}

// runOne drives one harness through one fixture, prints the line for it and records the
// row. It returns whether the harness failed the task — which a harness the profile
// excluded did not, because it was never asked.
func runOne(ctx context.Context, stdout *os.File, client *eval.Client, d eval.Driver,
	task eval.Tier2Task, props eval.ServerProps, cfg config, rep int) (failed bool, err error) {

	// The counters are the server's, not this run's, so anything else talking to the
	// endpoint while a harness works lands in its numbers. Runs are sequential for the
	// same reason the timings are.
	before, _ := client.Metrics(ctx)
	turns := client.CountTurns(ctx, turnPollInterval)
	res, work, err := eval.RunTier2(ctx, d, task, eval.Conditions{
		Desk: cfg.desk, Served: props, Sandbox: cfg.sandbox, Budget: cfg.budget, Keep: cfg.keep,
	})
	turnCount := turns.Stop()
	if err != nil {
		// A staging or fixture problem is not a result about the harness.
		return false, fmt.Errorf("%s: %s: %w", d.Name(), task.ID, err)
	}
	after, _ := client.Metrics(ctx)

	status := "PASS"
	switch {
	case res.Outcome == eval.Inadmissible:
		// Not a failure: the harness was never asked. Counting it as one would make a
		// profile's exclusions look like a suite the harnesses failed.
		status = "SKIP"
	case !res.Passed():
		status = "FAIL"
		failed = true
	}
	_, _ = fmt.Fprintf(stdout, "%-4s %-12s %-30s %s %s\n",
		status, d.Name(), res.TaskID, res.Outcome, res.Detail)
	spent := after.Sub(before)
	_, _ = fmt.Fprintf(stdout, "     %.1fs", res.WallSeconds)
	if spent.Available {
		_, _ = fmt.Fprintf(stdout, "  %d in (%d cached), %d out, %d turns",
			spent.PromptTokens, spent.CachedTokens, spent.PredictedTokens, turnCount)
	}
	_, _ = fmt.Fprintln(stdout)
	if cfg.keep {
		_, _ = fmt.Fprintf(stdout, "     scratch: %s\n", work)
	}
	if cfg.results == "" {
		return failed, nil
	}
	// Empty effort, and honestly so: tier-2 drives an external harness that builds its own
	// requests, so what it asked for is the harness's business and not something this
	// process can claim to have set.
	row := eval.NewRow(cfg.label, rep, "", "", eval.Sampling{}, props, "tier2", res)
	row.Harness, row.Profile = d.Name(), cfg.desk.Name
	row.Forced = cfg.forced
	row.Offline = cfg.sandbox != ""
	// What the run cost the server: ingested, reused from a held prefix, generated. Tier 1
	// reads the same three off a response body; a harness never shows this process one, so
	// they come off the counters instead.
	if spent.Available {
		row.PromptTokens = spent.PromptTokens
		row.CachedTokens = spent.CachedTokens
		row.CompletionTokens = spent.PredictedTokens
	}
	row.Turns = turnCount
	if err := eval.AppendRow(cfg.results, row); err != nil {
		return failed, fmt.Errorf("append result: %w", err)
	}
	return failed, nil
}

// loadTasks resolves what will be run. A fixture describes itself — what the bug is, which
// file carries it, which test grades it — so nothing here is typed at the command line and
// a row's task id is enough to find the instruction that produced it.
func loadTasks(c config) ([]eval.Tier2Task, error) {
	if c.fixture != "" {
		t, err := tier2Task(filepath.Join(c.fixture, "task.json"))
		if err != nil {
			return nil, err
		}
		return []eval.Tier2Task{t}, nil
	}
	paths, err := eval.DiscoverTasks(c.fixtures)
	if err != nil {
		return nil, err
	}
	var out []eval.Tier2Task
	for _, p := range paths {
		// A tool-call fixture is a single request by nature, so a suite scan passes over
		// what tier 2 cannot drive instead of refusing to start.
		t, err := tier2Task(p)
		if err != nil {
			continue
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no fixture under %s can be driven as tier 2", c.fixtures)
	}
	return out, nil
}

func tier2Task(path string) (eval.Tier2Task, error) {
	t, err := eval.LoadTask(path)
	if err != nil {
		return eval.Tier2Task{}, err
	}
	return eval.Tier2From(t)
}

// validate checks the run can be carried out and resolves every path to absolute. The
// absolute step is load-bearing: adapters run with the working directory set to the
// scratch checkout, so a relative config path resolves against /tmp instead of the repo.
func (c *config) validate() error {
	if (c.fixture == "") == (c.fixtures == "") {
		return errors.New("give exactly one of -fixture or -fixtures")
	}
	if len(c.drivers) == 0 {
		return errors.New("-drivers named none")
	}
	for _, p := range []*string{&c.fixture, &c.fixtures, &c.piExtension, &c.ocConfig, &c.ccEnv, &c.hermesCfg, &c.sandbox} {
		if *p == "" {
			continue
		}
		abs, err := filepath.Abs(*p)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", *p, err)
		}
		*p = abs
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
			// Each run gets a home of its own, seeded from this file: Hermes learns
			// across runs otherwise — see internal/harness/hermes.go.
			ds = append(ds, harness.NewHermes(c.hermesCfg))
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
