package eval

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Driver is the seam this package consumes: something that can be handed a checkout and
// an instruction and left to act.
//
// Declared here, in the consumer, rather than beside the adapters — and kept to two
// methods because that is all four harnesses genuinely agree on. They differ in almost
// everything else: where configuration lives, whether it is repo-local, what context
// window they will accept, where they keep what they learn. An interface wide enough to
// express those differences would have one implementation each and no seam at all.
type Driver interface {
	// Name identifies the driver in results. Stable, because it is a grouping key.
	Name() string
	// Drive runs the harness to completion. A non-nil error means the harness itself
	// failed — not that the task was done badly, which is what the fixture's own tests
	// are for.
	Drive(ctx context.Context, run Run) error
}

// Run is what a harness is handed: a checkout to work in, a directory of its own to keep
// state in, and what to do.
//
// StateDir is the second half of a cold run. Every candidate keeps something between
// runs — sessions, memories, skills learned from earlier work — and each has its own way
// of being pointed elsewhere for it. Left alone they accumulate across a sweep, and a
// suite then scores the order its fixtures came in as much as the harness.
type Run struct {
	Workdir     string
	StateDir    string
	Instruction string

	// SandboxProfile, when set, is a sandbox profile the harness is run under. It holds
	// the offline condition: the vision wants at least one path that works with no
	// network, and whether a harness has one is a scored outcome rather than a footnote.
	// Empty means the harness runs with the machine's own network.
	SandboxProfile string
}

// ContextFloorer is the optional half of Driver: a harness that refuses to run below a
// context window of its own choosing. Optional rather than a third method on Driver
// because one harness has a floor and the others do not, and a method every adapter but
// one answers zero to is a field with extra steps rather than a seam.
type ContextFloorer interface {
	// ContextFloor is the smallest context window, in tokens, the harness will accept.
	ContextFloor() int
}

// A DeskProfile is how the machine is being used while a harness is scored, and it caps
// the context that may be served. It is the machine's profile; Profile in this package is
// the model's, and the two are unrelated.
//
// Both ceilings are 0014's, measured against the desktop rather than against the model:
// every context up to 57,344 left the compositor working, and 65,536 stalled it for the
// whole run while still completing every request. So the two profiles differ in the
// desktop column alone, and the unattended ceiling is simply the largest context measured
// to serve.
type DeskProfile struct {
	Name    string
	Ceiling int
}

var deskProfiles = []DeskProfile{
	{Name: "attended", Ceiling: 57344},
	{Name: "unattended", Ceiling: 65536},
}

// LookupDeskProfile resolves a profile by name. There is no default: a tier-2 row that
// does not say which profile it was taken under cannot be compared with one that does.
func LookupDeskProfile(name string) (DeskProfile, error) {
	for _, p := range deskProfiles {
		if p.Name == name {
			return p, nil
		}
	}
	known := make([]string, 0, len(deskProfiles))
	for _, p := range deskProfiles {
		known = append(known, p.Name)
	}
	return DeskProfile{}, fmt.Errorf("unknown desk profile %q; known: %s", name, strings.Join(known, ", "))
}

// Excludes reports why a driver cannot be scored under this profile against this server,
// and returns "" when it can be. A driver that declares no floor is admitted everywhere.
//
// Two bounds can exclude it and they take different fixes, so the reason names which one
// bit: a floor above the profile's ceiling is a verdict — the machine cannot serve that
// harness while somebody is using it — while a floor above what the server is actually
// serving is a restart. A server that cannot be asked bounds nothing, which is the
// backend-without-/props case and is why the floor is checked against the profile first.
func (p DeskProfile) Excludes(d Driver, served ServerProps) string {
	f, ok := d.(ContextFloorer)
	if !ok {
		return ""
	}
	switch floor := f.ContextFloor(); {
	case floor > p.Ceiling:
		return fmt.Sprintf("context floor %d exceeds the %s ceiling %d", floor, p.Name, p.Ceiling)
	case served.Available && floor > served.NCtx:
		return fmt.Sprintf("context floor %d exceeds the %d this server is serving", floor, served.NCtx)
	}
	return ""
}

// Tier2Task is a fixture a harness is asked to fix. Unlike tier 1 it does not describe a
// single request: the harness decides how many turns to take and which tools to use, and
// the only thing scored is whether the result compiles and passes tests it never saw.
type Tier2Task struct {
	ID string
	// Dir holds the fixture. Source and TestFile carry a .txt suffix so the go tool
	// does not build this repo's deliberately-broken fixtures; they are written into
	// the scratch module under real .go names.
	Dir         string
	Source      string
	TestFile    string
	AnswerName  string // filename the source takes in the scratch module
	Instruction string
}

// bodyFence is the code block a tier-1 patch fixture shows the model inline. A harness
// opens the file instead, so the block comes out of the instruction — and only that block:
// a fixture may carry other fenced code that is part of the problem statement.
var bodyFence = regexp.MustCompile("(?s)\n*```(?:go|golang)?\\s*\n\\{\\{BODY\\}\\}\n```\n*")

// Tier2From turns a tier-1 patch fixture into a tier-2 one. Both tiers then pose the same
// problem from one source: the fixture's own user message, with the inlined source removed
// and the file it lives in named instead.
//
// A fixture without a tier2 block is not a defect — a tool-call fixture is a single request
// by nature — so callers scanning a suite skip what this refuses rather than failing.
func Tier2From(t *Task) (Tier2Task, error) {
	if t.Kind != "patch" || t.Patch == nil {
		return Tier2Task{}, fmt.Errorf("%s: only a patch fixture can be driven as tier 2", t.ID)
	}
	if t.Tier2 == nil || t.Tier2.AnswerName == "" {
		return Tier2Task{}, fmt.Errorf("%s: no tier2.answer_name, so the instruction cannot name a file", t.ID)
	}
	var user string
	for _, m := range t.Messages {
		if m.Role == "user" {
			user = m.Content
			break
		}
	}
	if !strings.Contains(user, "{{BODY}}") {
		return Tier2Task{}, fmt.Errorf("%s: user message inlines no {{BODY}} to remove", t.ID)
	}
	// The system message is deliberately dropped: it tells the model to reply with a whole
	// file in one block, which is the opposite of what a harness is asked to do.
	return Tier2Task{
		ID:          t.ID,
		Dir:         t.Patch.Dir,
		Source:      t.Patch.Source,
		TestFile:    t.Patch.TestFile,
		AnswerName:  t.Tier2.AnswerName,
		Instruction: fmt.Sprintf("In %s: %s", t.Tier2.AnswerName, strings.TrimSpace(bodyFence.ReplaceAllString(user, "\n\n"))),
	}, nil
}

// ErrFixtureNotBroken means the fixture passed its own tests before the harness touched
// it. That makes the task worthless — every driver would "pass" it — so it is a hard
// error rather than a result. This check exists because a tier-1 fixture once punished
// correct behaviour and was only caught by running it.
var ErrFixtureNotBroken = errors.New("fixture passes its own tests before the harness runs")

// Conditions are the terms a run is conducted under, as opposed to what is being run:
// which desk profile its numbers count against, what the server reports serving, whether
// the harness is denied the network, and whether the checkout survives for inspection.
type Conditions struct {
	Desk    DeskProfile
	Served  ServerProps
	Sandbox string // sandbox profile the harness runs under; empty leaves it online
	Budget  time.Duration
	Keep    bool
}

// DefaultBudget bounds one tier-2 run. Generous against what a harness legitimately takes
// here — the slowest honest run measured five minutes, most take one — and bounded because
// a harness that loops does not stop on its own: one spent 45 minutes and 90 turns on a
// fixture the others finished in three, which at 15 fixtures a sweep is a day of machine
// time for no result.
const DefaultBudget = 10 * time.Minute

// RunTier2 stages the fixture in a scratch module, hands it to the driver, and scores the
// outcome by running the fixture's own tests. It returns the scratch directory, which is
// removed unless the conditions keep it.
func RunTier2(ctx context.Context, d Driver, t Tier2Task, c Conditions) (Result, string, error) {
	res := Result{TaskID: t.ID}

	// Checked before anything is staged, and returned as a result rather than an error:
	// scoring a harness at a context it refuses produces either a quality figure for a
	// configuration nobody can use or a harness failure that reads as the model
	// answering badly. Both are worse than having no figure at all.
	if why := c.Desk.Excludes(d, c.Served); why != "" {
		res.Outcome, res.Detail = Inadmissible, why
		return res, "", nil
	}

	work, err := os.MkdirTemp("", "localcode-tier2-")
	if err != nil {
		return res, "", fmt.Errorf("scratch dir: %w", err)
	}
	if !c.Keep {
		defer func() { _ = os.RemoveAll(work) }()
	}

	if err := stageFixture(t, work); err != nil {
		return res, work, err
	}
	if err := stageTest(t, work); err != nil {
		return res, work, err
	}

	// A fixture that already passes cannot measure anything.
	if out, err := goTest(ctx, work); err == nil {
		return res, work, ErrFixtureNotBroken
	} else if isBuildFailure(out) {
		return res, work, fmt.Errorf("fixture does not compile before the run: %s", firstUseful(string(out)))
	}

	// Each run gets a state directory of its own, thrown away with the checkout.
	state, err := os.MkdirTemp("", "localcode-tier2-state-")
	if err != nil {
		return res, work, fmt.Errorf("state dir: %w", err)
	}
	if !c.Keep {
		defer func() { _ = os.RemoveAll(state) }()
	}

	// The test is taken away for the duration of the run and put back to score with. It
	// has to be present for the check above and absent for the harness, and "a test the
	// harness never saw" is otherwise just a description of a file sitting in its working
	// directory: every candidate here has a Read tool, and they differ in how much of the
	// directory they look at, which would make the leak a per-harness advantage.
	if err := os.Remove(filepath.Join(work, testName)); err != nil {
		return res, work, fmt.Errorf("withhold the test: %w", err)
	}

	// Memory across the run, for the same reason tier 1 records it: a run whose swap grew
	// measured the pager. It matters more here — a tier-2 run is minutes of a harness and
	// a model working together, at a context the desktop was already measured to strain.
	memBefore := sampleMemory()
	budget := c.Budget
	if budget <= 0 {
		budget = DefaultBudget
	}
	runCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	start := time.Now()
	driveErr := d.Drive(runCtx, Run{
		Workdir: work, StateDir: state, Instruction: t.Instruction, SandboxProfile: c.Sandbox,
	})
	res.WallSeconds = time.Since(start).Seconds()
	memAfter := sampleMemory()
	res.FreeGB = memAfter.FreeGB
	res.SwapDeltaMB = memAfter.SwapUsedMB - memBefore.SwapUsedMB
	res.MemMeasured = memBefore.OK && memAfter.OK

	if driveErr != nil {
		// Out of budget is not a broken adapter and not a wrong answer: the harness was
		// still working when its clock ran out. Kept distinct because the two take
		// different fixes — one is a bug, the other is a harness that does not converge,
		// which is a result about the harness and one this comparison is looking for.
		if runCtx.Err() != nil && ctx.Err() == nil {
			res.Outcome = FailOverBudget
			res.Detail = fmt.Sprintf("still working after its %s budget", budget)
			return res, work, nil
		}
		// The harness failed, which is not the model answering badly. Kept distinct so a
		// broken adapter is never recorded as a quality result.
		res.Outcome, res.Detail = FailServer, truncate(driveErr.Error(), 200)
		return res, work, nil
	}

	// A harness that wrote nothing where it was pointed kept its state where it always
	// does, which is the machine's own — so this run inherited whatever the last one
	// left. That cannot be scored as a cold result, and it is the harness's doing rather
	// than the model's.
	if entries, err := os.ReadDir(state); err != nil || len(entries) == 0 {
		res.Outcome = FailServer
		res.Detail = "harness wrote nothing to the state directory it was given, so the run cannot be called cold"
		return res, work, nil
	}

	// Written back unconditionally, so a harness that left a file of this name behind is
	// scored against the fixture's test rather than its own.
	if err := stageTest(t, work); err != nil {
		return res, work, err
	}

	out, err := goTest(ctx, work)
	switch {
	case err == nil:
		res.Outcome = Pass
	case isBuildFailure(out):
		res.Outcome, res.Detail = FailCompile, truncate(firstUseful(string(out)), 200)
	default:
		res.Outcome, res.Detail = FailTest, truncate(firstUseful(string(out)), 200)
	}
	return res, work, nil
}

// testName is what the fixture's test is called inside the scratch module. Fixed rather
// than derived from the fixture: the runner takes it away and puts it back, and both ends
// of that have to name the same file.
const testName = "verify_test.go"

// stageFixture writes the module and the broken source — everything the harness is meant
// to see. The test is staged separately by stageTest.
func stageFixture(t Tier2Task, work string) error {
	src, err := os.ReadFile(filepath.Join(t.Dir, t.Source))
	if err != nil {
		return fmt.Errorf("fixture source: %w", err)
	}
	answer := t.AnswerName
	if answer == "" {
		answer = "answer.go"
	}
	files := map[string][]byte{
		"go.mod": []byte("module scratch\n\ngo 1.26\n"),
		answer:   src,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(work, name), data, 0o644); err != nil {
			return fmt.Errorf("stage %s: %w", name, err)
		}
	}
	return nil
}

// stageTest writes the fixture's test into the scratch module, overwriting whatever is
// there. Called twice: once to prove the fixture is broken, once to score with.
func stageTest(t Tier2Task, work string) error {
	test, err := os.ReadFile(filepath.Join(t.Dir, t.TestFile))
	if err != nil {
		return fmt.Errorf("fixture test: %w", err)
	}
	if err := os.WriteFile(filepath.Join(work, testName), test, 0o644); err != nil {
		return fmt.Errorf("stage %s: %w", testName, err)
	}
	return nil
}

func goTest(ctx context.Context, dir string) ([]byte, error) {
	runCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "go", "test", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local")
	return cmd.CombinedOutput()
}

func isBuildFailure(out []byte) bool {
	text := string(out)
	for _, marker := range []string{"[build failed]", "syntax error", "undefined:", "cannot use"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
