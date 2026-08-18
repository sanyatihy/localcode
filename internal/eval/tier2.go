package eval

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Driver is the seam this package consumes: something that can be pointed at a working
// directory with an instruction and left to act.
//
// Declared here, in the consumer, rather than beside the adapters — and kept to two
// methods because that is all three harnesses genuinely agree on. They differ in almost
// everything else: where configuration lives, whether it is repo-local, what context
// window they will accept. An interface wide enough to express those differences would
// have one implementation each and no seam at all.
type Driver interface {
	// Name identifies the driver in results. Stable, because it is a grouping key.
	Name() string
	// Drive runs the harness to completion against workdir. A non-nil error means the
	// harness itself failed — not that the task was done badly, which is what the
	// fixture's own tests are for.
	Drive(ctx context.Context, workdir, instruction string) error
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

// ErrFixtureNotBroken means the fixture passed its own tests before the harness touched
// it. That makes the task worthless — every driver would "pass" it — so it is a hard
// error rather than a result. This check exists because a tier-1 fixture once punished
// correct behaviour and was only caught by running it.
var ErrFixtureNotBroken = errors.New("fixture passes its own tests before the harness runs")

// RunTier2 stages the fixture in a scratch module, hands it to the driver, and scores the
// outcome by running the fixture's own tests. keep leaves the scratch directory in place
// for inspection and returns its path.
func RunTier2(ctx context.Context, d Driver, t Tier2Task, keep bool) (Result, string, error) {
	res := Result{TaskID: t.ID}

	work, err := os.MkdirTemp("", "localcode-tier2-")
	if err != nil {
		return res, "", fmt.Errorf("scratch dir: %w", err)
	}
	if !keep {
		defer func() { _ = os.RemoveAll(work) }()
	}

	if err := stageFixture(t, work); err != nil {
		return res, work, err
	}

	// A fixture that already passes cannot measure anything.
	if out, err := goTest(ctx, work); err == nil {
		return res, work, ErrFixtureNotBroken
	} else if isBuildFailure(out) {
		return res, work, fmt.Errorf("fixture does not compile before the run: %s", firstUseful(string(out)))
	}

	start := time.Now()
	driveErr := d.Drive(ctx, work, t.Instruction)
	res.WallSeconds = time.Since(start).Seconds()

	if driveErr != nil {
		// The harness failed, which is not the model answering badly. Kept distinct so a
		// broken adapter is never recorded as a quality result.
		res.Outcome, res.Detail = FailServer, truncate(driveErr.Error(), 200)
		return res, work, nil
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

func stageFixture(t Tier2Task, work string) error {
	src, err := os.ReadFile(filepath.Join(t.Dir, t.Source))
	if err != nil {
		return fmt.Errorf("fixture source: %w", err)
	}
	test, err := os.ReadFile(filepath.Join(t.Dir, t.TestFile))
	if err != nil {
		return fmt.Errorf("fixture test: %w", err)
	}
	answer := t.AnswerName
	if answer == "" {
		answer = "answer.go"
	}
	files := map[string][]byte{
		"go.mod":         []byte("module scratch\n\ngo 1.26\n"),
		answer:           src,
		"verify_test.go": test,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(work, name), data, 0o644); err != nil {
			return fmt.Errorf("stage %s: %w", name, err)
		}
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
