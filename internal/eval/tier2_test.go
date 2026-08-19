package eval

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeDriver is a hand-written double rather than a generated mock: it records what it
// was asked to do and performs a scripted edit, which is all the runner needs to be
// exercised. A real harness cannot be asked to fail in a specific way on demand.
type fakeDriver struct {
	name    string
	err     error
	writes  map[string]string // filename -> contents, applied to the workdir
	gotDir  string
	gotText string
	calls   int
}

func (f *fakeDriver) Name() string { return f.name }

func (f *fakeDriver) Drive(_ context.Context, workdir, instruction string) error {
	f.calls++
	f.gotDir, f.gotText = workdir, instruction
	if f.err != nil {
		return f.err
	}
	for name, body := range f.writes {
		if err := os.WriteFile(filepath.Join(workdir, name), []byte(body), 0o644); err != nil {
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
			res, _, err := RunTier2(context.Background(), tc.driver, brokenFixture(t), unattended(t), ServerProps{}, false)
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
	_, _, err := RunTier2(context.Background(), d, task, unattended(t), ServerProps{}, false)
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

	_, work, err := RunTier2(context.Background(), d, task, unattended(t), ServerProps{}, true)
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

	res, _, err := RunTier2(context.Background(), d, brokenFixture(t), attended, ServerProps{}, false)
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

	res, _, err := RunTier2(context.Background(), d, brokenFixture(t), p, ServerProps{}, false)
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

	res, _, err := RunTier2(context.Background(), d, brokenFixture(t), p, served, false)
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

	res, _, err := RunTier2(context.Background(), d, brokenFixture(t), p, ServerProps{}, false)
	if err != nil {
		t.Fatalf("RunTier2: %v", err)
	}
	if res.Outcome != Pass {
		t.Errorf("outcome = %s (%s), want %s", res.Outcome, res.Detail, Pass)
	}
}
