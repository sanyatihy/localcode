package eval

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeDriver is a hand-written double rather than a generated mock: it records what it
// was asked to do and performs a scripted edit, which is all the runner needs to be
// exercised. A real harness cannot be asked to fail in a specific way on demand.
type fakeDriver struct {
	name     string
	err      error
	writes   map[string]string // filename -> contents, applied to the workdir
	gotDir   string
	gotText  string
	gotState string
	saw      []string // what was in the workdir when the harness was handed it
	calls    int

	// keepsNoState stands in for a harness that ignored the state directory it was
	// given and kept its sessions on the machine instead.
	keepsNoState bool
}

func (f *fakeDriver) Name() string { return f.name }

func (f *fakeDriver) Drive(_ context.Context, r Run) error {
	f.calls++
	f.gotDir, f.gotText, f.gotState = r.Workdir, r.Instruction, r.StateDir
	if entries, err := os.ReadDir(r.Workdir); err == nil {
		f.saw = nil
		for _, e := range entries {
			f.saw = append(f.saw, e.Name())
		}
	}
	if f.err != nil {
		return f.err
	}
	// Every real harness writes where it is pointed, and the runner refuses to call a
	// run cold when nothing did. A fake that wrote nothing would fail every test for
	// the wrong reason, so it keeps state like the harnesses it stands in for.
	if !f.keepsNoState {
		if err := os.WriteFile(filepath.Join(r.StateDir, "session"), []byte("x"), 0o644); err != nil {
			return err
		}
	}
	for name, body := range f.writes {
		if err := os.WriteFile(filepath.Join(r.Workdir, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// flooredDriver is a fake harness that refuses to run below a context window of its own,
// which is the shape Hermes has. fakeDriver deliberately declares no floor, so the two
// together cover both sides of the optional interface.
type flooredDriver struct {
	fakeDriver
	floor int
}

func (f *flooredDriver) ContextFloor() int { return f.floor }

// The fakes here declare no floor of their own, so any profile admits them; naming a real
// one keeps the tests reading like an invocation somebody would type.
func unattended(t *testing.T) DeskProfile {
	t.Helper()
	p, err := LookupDeskProfile("unattended")
	if err != nil {
		t.Fatalf("LookupDeskProfile: %v", err)
	}
	return p
}

// A fixture whose source is broken and whose test catches it — the shape every tier-2
// task must have.
func brokenFixture(t *testing.T) Tier2Task {
	t.Helper()
	dir := t.TempDir()
	src := `package main

var ErrNil = errStr("nil")

type errStr string

func (e errStr) Error() string { return string(e) }

func Head(xs []int) (int, error) { return xs[0], nil }
`
	test := `package main

import "testing"

func TestHeadEmpty(t *testing.T) {
	if _, err := Head(nil); err != ErrNil {
		t.Fatalf("Head(nil) err = %v, want ErrNil", err)
	}
}

func TestHead(t *testing.T) {
	if v, err := Head([]int{7}); v != 7 || err != nil {
		t.Fatalf("Head([7]) = %v, %v", v, err)
	}
}
`
	write(t, filepath.Join(dir, "broken.go.txt"), src)
	write(t, filepath.Join(dir, "verify_test.go.txt"), test)
	return Tier2Task{
		ID: "fixture", Dir: dir, Source: "broken.go.txt", TestFile: "verify_test.go.txt",
		AnswerName: "head.go", Instruction: "guard the empty case",
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

const fixedSource = `package main

var ErrNil = errStr("nil")

type errStr string

func (e errStr) Error() string { return string(e) }

func Head(xs []int) (int, error) {
	if len(xs) == 0 {
		return 0, ErrNil
	}
	return xs[0], nil
}
`

func TestRunTier2Outcomes(t *testing.T) {
	tests := []struct {
		name   string
		driver *fakeDriver
		want   Outcome
	}{
		{
			name:   "correct fix passes the unseen test",
			driver: &fakeDriver{name: "fake", writes: map[string]string{"head.go": fixedSource}},
			want:   Pass,
		},
		{
			name:   "harness does nothing, so the fixture still fails",
			driver: &fakeDriver{name: "fake"},
			want:   FailTest,
		},
		{
			name: "harness writes code that does not compile",
			driver: &fakeDriver{name: "fake", writes: map[string]string{
				"head.go": "package main\n\nfunc Head(xs []int) (int, error) { return xs[0] }\n"}},
			want: FailCompile,
		},
		{
			name:   "harness itself fails",
			driver: &fakeDriver{name: "fake", err: errors.New("exit status 1")},
			want:   FailServer,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, _, err := RunTier2(context.Background(), tc.driver, brokenFixture(t), Conditions{Desk: unattended(t)})
			if err != nil {
				t.Fatalf("RunTier2: %v", err)
			}
			if res.Outcome != tc.want {
				t.Errorf("outcome = %s (%s), want %s", res.Outcome, res.Detail, tc.want)
			}
			if tc.driver.calls != 1 {
				t.Errorf("driver called %d times, want 1", tc.driver.calls)
			}
		})
	}
}

// A fixture that already passes would score every driver as correct and mean nothing.
// This is the check a tier-1 fixture once needed and did not have.
func TestRunTier2RejectsAFixtureThatIsNotBroken(t *testing.T) {
	task := brokenFixture(t)
	write(t, filepath.Join(task.Dir, task.Source), fixedSource)

	d := &fakeDriver{name: "fake"}
	_, _, err := RunTier2(context.Background(), d, task, Conditions{Desk: unattended(t)})
	if !errors.Is(err, ErrFixtureNotBroken) {
		t.Fatalf("err = %v, want ErrFixtureNotBroken", err)
	}
	if d.calls != 0 {
		t.Errorf("driver was called %d times; a useless fixture must be rejected before the run", d.calls)
	}
}

func TestRunTier2StagesTheWorkdirForTheDriver(t *testing.T) {
	d := &fakeDriver{name: "fake", writes: map[string]string{"head.go": fixedSource}}
	task := brokenFixture(t)

	_, work, err := RunTier2(context.Background(), d, task, Conditions{Desk: unattended(t), Keep: true})
	if err != nil {
		t.Fatalf("RunTier2: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(work) })

	if d.gotDir != work {
		t.Errorf("driver got workdir %q, want %q", d.gotDir, work)
	}
	if d.gotText != task.Instruction {
		t.Errorf("driver got instruction %q, want %q", d.gotText, task.Instruction)
	}
	// The unseen test must be staged under a real .go name, and the fixture's .txt
	// suffix must not leak into the scratch module.
	for _, want := range []string{"go.mod", "head.go", "verify_test.go"} {
		if _, err := os.Stat(filepath.Join(work, want)); err != nil {
			t.Errorf("scratch module missing %s", want)
		}
	}
	if _, err := os.Stat(filepath.Join(work, "broken.go.txt")); err == nil {
		t.Error("fixture .txt file leaked into the scratch module")
	}
}

// A harness whose floor is above the profile's ceiling is excluded, and the exclusion is a
// row rather than a gap: scored anyway it would produce a quality figure for a context the
// machine cannot serve while somebody is using it, and left out entirely its absence would
// be indistinguishable from a run nobody got round to.
func TestRunTier2ExcludesAHarnessTheProfileCannotServe(t *testing.T) {
	attended, err := LookupDeskProfile("attended")
	if err != nil {
		t.Fatalf("LookupDeskProfile: %v", err)
	}
	d := &flooredDriver{fakeDriver: fakeDriver{name: "floored"}, floor: attended.Ceiling + 1}

	res, _, err := RunTier2(context.Background(), d, brokenFixture(t), Conditions{Desk: attended})
	if err != nil {
		t.Fatalf("RunTier2: %v", err)
	}
	if res.Outcome != Inadmissible {
		t.Errorf("outcome = %s, want %s", res.Outcome, Inadmissible)
	}
	// The detail carries both numbers, because "not admissible" without them leaves the
	// reader unable to tell a harness that just misses from one that cannot ever fit.
	for _, want := range []string{
		strconv.Itoa(attended.Ceiling + 1), attended.Name, strconv.Itoa(attended.Ceiling),
	} {
		if !strings.Contains(res.Detail, want) {
			t.Errorf("detail %q does not name %q", res.Detail, want)
		}
	}
	if d.calls != 0 {
		t.Errorf("driver was called %d times; an excluded harness must not be run", d.calls)
	}
}

// The same harness under the profile whose ceiling clears its floor is scored normally.
func TestRunTier2ScoresAFlooredHarnessThatFits(t *testing.T) {
	p := unattended(t)
	d := &flooredDriver{
		fakeDriver: fakeDriver{name: "floored", writes: map[string]string{"head.go": fixedSource}},
		floor:      p.Ceiling,
	}

	res, _, err := RunTier2(context.Background(), d, brokenFixture(t), Conditions{Desk: p})
	if err != nil {
		t.Fatalf("RunTier2: %v", err)
	}
	if res.Outcome != Pass {
		t.Errorf("outcome = %s (%s), want %s", res.Outcome, res.Detail, Pass)
	}
	if d.calls != 1 {
		t.Errorf("driver called %d times, want 1", d.calls)
	}
}

// An unknown profile is refused rather than defaulted: a row that does not say which
// profile it was taken under cannot be compared with one that does, and the two ceilings
// differ by exactly the thing under test.
func TestLookupDeskProfileRefusesWhatItDoesNotKnow(t *testing.T) {
	if _, err := LookupDeskProfile("idle"); err == nil {
		t.Fatal("expected an error naming the profiles that exist")
	}
	for _, name := range []string{"attended", "unattended"} {
		p, err := LookupDeskProfile(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if p.Ceiling <= 0 {
			t.Errorf("%s: ceiling = %d", name, p.Ceiling)
		}
	}
}

// A server serving less than a harness's floor excludes it too, and for a different
// reason than the profile does: the fix is a restart, not a verdict about the machine.
// Driven anyway, the harness refuses and the fixture stays broken — which is scored
// fail_test_failed and is indistinguishable from the model answering badly.
func TestRunTier2ExcludesAHarnessTheServerIsNotServingFor(t *testing.T) {
	p := unattended(t)
	d := &flooredDriver{fakeDriver: fakeDriver{name: "floored"}, floor: p.Ceiling}
	served := ServerProps{NCtx: 32768, ModelPath: "/models/m.gguf", Available: true}

	res, _, err := RunTier2(context.Background(), d, brokenFixture(t), Conditions{Desk: p, Served: served})
	if err != nil {
		t.Fatalf("RunTier2: %v", err)
	}
	if res.Outcome != Inadmissible {
		t.Fatalf("outcome = %s, want %s", res.Outcome, Inadmissible)
	}
	if !strings.Contains(res.Detail, strconv.Itoa(served.NCtx)) {
		t.Errorf("detail %q does not name what the server serves", res.Detail)
	}
	if d.calls != 0 {
		t.Errorf("driver was called %d times; an excluded harness must not be run", d.calls)
	}
}

// A backend that cannot be asked what it serves bounds nothing. It is still scoreable —
// MLX serves completions without llama.cpp's /props — so the floor is checked against the
// profile alone rather than against a confident zero that would exclude every harness.
func TestRunTier2ScoresWhenTheServerCannotBeAsked(t *testing.T) {
	p := unattended(t)
	d := &flooredDriver{
		fakeDriver: fakeDriver{name: "floored", writes: map[string]string{"head.go": fixedSource}},
		floor:      p.Ceiling,
	}

	res, _, err := RunTier2(context.Background(), d, brokenFixture(t), Conditions{Desk: p})
	if err != nil {
		t.Fatalf("RunTier2: %v", err)
	}
	if res.Outcome != Pass {
		t.Errorf("outcome = %s (%s), want %s", res.Outcome, res.Detail, Pass)
	}
}

// The unseen test must not be in the working directory while the harness runs. Every
// candidate has a Read tool and they differ in how much of the directory they read, so a
// test left lying there is both a leak and a per-harness one.
func TestRunTier2WithholdsTheTestFromTheHarness(t *testing.T) {
	d := &fakeDriver{name: "fake", writes: map[string]string{"head.go": fixedSource}}

	res, _, err := RunTier2(context.Background(), d, brokenFixture(t), Conditions{Desk: unattended(t)})
	if err != nil {
		t.Fatalf("RunTier2: %v", err)
	}
	if slices.Contains(d.saw, testName) {
		t.Errorf("the harness was handed %v, which includes the test it is scored by", d.saw)
	}
	// Withholding it must not cost the run its scoring: it comes back to grade with.
	if res.Outcome != Pass {
		t.Errorf("outcome = %s (%s), want %s", res.Outcome, res.Detail, Pass)
	}
}

// A harness that writes a file where the test goes is scored by the fixture's test, not by
// its own. Without the write-back this passes anything: the harness supplies both the
// answer and the marking.
func TestRunTier2ScoresAgainstTheFixturesTestNotTheHarnesss(t *testing.T) {
	permissive := `package main

import "testing"

func TestNothing(t *testing.T) {}
`
	d := &fakeDriver{name: "fake", writes: map[string]string{testName: permissive}}

	res, _, err := RunTier2(context.Background(), d, brokenFixture(t), Conditions{Desk: unattended(t)})
	if err != nil {
		t.Fatalf("RunTier2: %v", err)
	}
	if res.Outcome != FailTest {
		t.Errorf("outcome = %s (%s), want %s — the harness marked its own work",
			res.Outcome, res.Detail, FailTest)
	}
}

// The instruction a harness gets is derived from the fixture's own user message, so the two
// tiers cannot drift into posing different problems. What comes out must lose the inlined
// source, name the file it went into, and keep any other code the statement needs.
func TestTier2FromDerivesTheInstruction(t *testing.T) {
	task := &Task{
		ID: "patch-x", Kind: "patch",
		Patch: &Patch{Dir: "d", Source: "broken.go.txt", TestFile: "verify_test.go.txt"},
		Tier2: &Tier2Spec{AnswerName: "round.go"},
		Messages: []Message{
			{Role: "system", Content: "Reply with the complete corrected file in a single ```go block."},
			{Role: "user", Content: "RoundHalf truncates.\n\n```go\n{{BODY}}\n```\n\nThis test keeps passing:\n\n```go\nfunc TestExisting(t *testing.T) {}\n```"},
		},
	}
	got, err := Tier2From(task)
	if err != nil {
		t.Fatalf("Tier2From: %v", err)
	}
	if strings.Contains(got.Instruction, "{{BODY}}") {
		t.Errorf("instruction still inlines the source: %q", got.Instruction)
	}
	if !strings.HasPrefix(got.Instruction, "In round.go: RoundHalf truncates.") {
		t.Errorf("instruction does not name the file it is about: %q", got.Instruction)
	}
	if !strings.Contains(got.Instruction, "func TestExisting") {
		t.Errorf("instruction dropped code the statement needs: %q", got.Instruction)
	}
	// The tier-1 system message tells the model to answer with a whole file in one
	// block, which is not what a harness is being asked to do.
	if strings.Contains(got.Instruction, "single ```go block") {
		t.Errorf("instruction carries the tier-1 system message: %q", got.Instruction)
	}
	if got.AnswerName != "round.go" || got.Source != "broken.go.txt" || got.Dir != "d" {
		t.Errorf("fixture fields not carried over: %+v", got)
	}
}

// What tier 2 cannot drive is refused by name, so a suite scan can pass over it and a
// single-fixture run says why rather than driving something meaningless.
func TestTier2FromRefusesWhatItCannotDrive(t *testing.T) {
	patch := &Patch{Dir: "d", Source: "broken.go.txt", TestFile: "verify_test.go.txt"}
	body := []Message{{Role: "user", Content: "fix it\n\n```go\n{{BODY}}\n```"}}
	for _, tc := range []struct {
		name string
		task *Task
	}{
		{"a tool-call fixture is one request by nature", &Task{ID: "t", Kind: "toolcall", Messages: body}},
		{"a patch fixture with no tier2 block names no file", &Task{ID: "t", Kind: "patch", Patch: patch, Messages: body}},
		{"a fixture that inlines nothing was never a patch task", &Task{
			ID: "t", Kind: "patch", Patch: patch, Tier2: &Tier2Spec{AnswerName: "x.go"},
			Messages: []Message{{Role: "user", Content: "fix it"}}}},
	} {
		if _, err := Tier2From(tc.task); err == nil {
			t.Errorf("%s: expected an error", tc.name)
		}
	}
}

// Every committed patch fixture must be drivable, because the suite a harness is scored on
// is exactly these and a fixture that silently drops out shortens the comparison.
func TestEveryCommittedPatchFixtureIsDrivable(t *testing.T) {
	paths, err := DiscoverTasks("../../tasks")
	if err != nil {
		t.Fatalf("DiscoverTasks: %v", err)
	}
	found := 0
	for _, p := range paths {
		task, err := LoadTask(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if task.Kind != "patch" {
			continue
		}
		found++
		t2, err := Tier2From(task)
		if err != nil {
			t.Errorf("%s: %v", task.ID, err)
			continue
		}
		if !strings.Contains(t2.Instruction, t2.AnswerName) {
			t.Errorf("%s: instruction never names %s", task.ID, t2.AnswerName)
		}
	}
	if found == 0 {
		t.Fatal("no patch fixtures found; this test would pass vacuously")
	}
}

// A harness is handed a state directory of its own, and one that writes nothing there kept
// its state where it always does — on the machine, carried over from the last run. That
// cannot be scored as cold, and it is the harness's doing rather than the model's.
func TestRunTier2RefusesARunItCannotCallCold(t *testing.T) {
	d := &fakeDriver{name: "fake", keepsNoState: true, writes: map[string]string{"head.go": fixedSource}}

	res, _, err := RunTier2(context.Background(), d, brokenFixture(t), Conditions{Desk: unattended(t)})
	if err != nil {
		t.Fatalf("RunTier2: %v", err)
	}
	if res.Outcome != FailServer {
		t.Errorf("outcome = %s, want %s — a fix that cannot be called cold is not a pass",
			res.Outcome, FailServer)
	}
	if !strings.Contains(res.Detail, "cold") {
		t.Errorf("detail %q does not say why", res.Detail)
	}
}

// The state directory is separate from the checkout: a harness that kept its sessions
// inside the working directory would be handing the next fixture its own notes, and the
// files would show up as work the harness did.
func TestRunTier2KeepsStateOutOfTheCheckout(t *testing.T) {
	d := &fakeDriver{name: "fake", writes: map[string]string{"head.go": fixedSource}}

	if _, _, err := RunTier2(context.Background(), d, brokenFixture(t), Conditions{Desk: unattended(t)}); err != nil {
		t.Fatalf("RunTier2: %v", err)
	}
	if d.gotState == "" {
		t.Fatal("the harness was given no state directory")
	}
	if d.gotState == d.gotDir || strings.HasPrefix(d.gotState, d.gotDir+string(filepath.Separator)) {
		t.Errorf("state directory %q is inside the checkout %q", d.gotState, d.gotDir)
	}
}

// A harness still working when its budget expires is over budget, not broken: the two
// take different fixes, and an unbounded sweep is what a looping harness turns into — one
// spent 45 minutes and 90 turns on a fixture the others finished in three.
func TestRunTier2StopsAHarnessThatRunsPastItsBudget(t *testing.T) {
	d := &slowDriver{}

	res, _, err := RunTier2(context.Background(), d, brokenFixture(t),
		Conditions{Desk: unattended(t), Budget: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("RunTier2: %v", err)
	}
	if res.Outcome != FailOverBudget {
		t.Errorf("outcome = %s (%s), want %s", res.Outcome, res.Detail, FailOverBudget)
	}
	if !strings.Contains(res.Detail, "budget") {
		t.Errorf("detail %q does not say what stopped it", res.Detail)
	}
	// The clock is the finding when a harness runs long, so it has to be recorded.
	if res.WallSeconds <= 0 {
		t.Error("an over-budget run recorded no wall time")
	}
}

// slowDriver works until it is stopped, which is what a looping harness looks like from
// outside: it does not fail, it does not finish.
type slowDriver struct{}

func (s *slowDriver) Name() string { return "slow" }

func (s *slowDriver) Drive(ctx context.Context, r Run) error {
	if err := os.WriteFile(filepath.Join(r.StateDir, "session"), []byte("x"), 0o644); err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}
