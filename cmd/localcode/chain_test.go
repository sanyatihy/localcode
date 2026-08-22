package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sanyatihy/localcode/internal/chain"
)

// chainStub puts a claude on PATH that writes a scripted handoff into the directory it was
// given, a different one per invocation. What a supervisor does with a handoff is
// arithmetic; only producing one needs a model, so this is where the model stops being
// needed and the decisions start being testable.
func chainStub(t *testing.T, handoffs ...string) {
	t.Helper()
	dir := t.TempDir()
	count := filepath.Join(dir, "invocations")
	var cases strings.Builder
	for i, h := range handoffs {
		fmt.Fprintf(&cases, "%d)\ncat > \"$out\" <<'HANDOFF_EOF'\n%s\nHANDOFF_EOF\n;;\n", i+1, h)
	}
	body := "#!/bin/sh\n" +
		"n=$(cat '" + count + "' 2>/dev/null || echo 0); n=$((n+1)); echo $n > '" + count + "'\n" +
		"out=/dev/null\n" +
		"while [ $# -gt 0 ]; do\n" +
		"  if [ \"$1\" = \"--add-dir\" ]; then out=\"$2/HANDOFF.md\"; fi\n" +
		"  shift\n" +
		"done\n" +
		"case $n in\n" + cases.String() + "esac\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func handoffSaying(next string) string {
	return "# Handoff\n\n**Box:** fix the failing tests\n**Files:** `median.go`, and the test that " +
		"names it\n**Tried:** the even-length case now averages the two middle values\n" +
		"**Next:** " + next + "\n"
}

// runHere runs a chain in a fresh repository and returns the exit code and the state
// directory the chain was kept in.
func runHere(t *testing.T, o opts) (int, string) {
	t.Helper()
	repo := t.TempDir()
	t.Chdir(repo)
	o.endpoint, o.noServe = healthy(t, http.StatusOK), true
	if o.ceiling == 0 {
		o.ceiling, o.calls, o.sessions = 50, 30, 8
	}
	code, err := run(o)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	state, err := repoState(repo)
	if err != nil {
		t.Fatal(err)
	}
	return code, state
}

func chainDirOf(t *testing.T, state string) string {
	t.Helper()
	ids, err := chain.Chains(state)
	if err != nil || len(ids) != 1 {
		t.Fatalf("chains: %v %v", ids, err)
	}
	return filepath.Join(state, "chains", ids[0])
}

// The point of the feature: work no single session could hold finishes across several,
// and `none` is the only thing that says it is finished.
func TestChainRunsUntilAHandoffSaysThereIsNothingLeft(t *testing.T) {
	root := fakeCheckout(t)
	chainStub(t, handoffSaying("fix Clamp"), handoffSaying("fix Title"), handoffSaying("none"))
	passthroughSandbox(t)

	code, state := runHere(t, opts{checkout: root, args: []string{"fix the four tests"}})
	if code != 0 {
		t.Fatalf("a finished chain must exit 0, got %d", code)
	}
	dir := chainDirOf(t, state)
	if got := chain.NextSession(dir); got != 4 {
		t.Fatalf("the chain must have run three sessions, next is %d", got)
	}
	// Each session in its own directory, or the handoff the next one inherits is the file
	// it is about to overwrite.
	for _, n := range []string{"01", "02", "03"} {
		if _, err := os.Stat(filepath.Join(dir, n, chain.HandoffName)); err != nil {
			t.Fatalf("session %s left no handoff: %v", n, err)
		}
	}
}

// A chain still writing handoffs is not a chain making progress. Two sessions planning the
// same step is the shape it fails in, and the file is what a person needs to see next.
func TestChainStopsWhenTwoSessionsPlanTheSameThing(t *testing.T) {
	root := fakeCheckout(t)
	chainStub(t, handoffSaying("fix Clamp"), handoffSaying("fix Clamp"), handoffSaying("none"))
	passthroughSandbox(t)

	code, state := runHere(t, opts{checkout: root, args: []string{"fix the four tests"}})
	if code != 1 {
		t.Fatalf("a stalled chain must exit 1, got %d", code)
	}
	if got := chain.NextSession(chainDirOf(t, state)); got != 3 {
		t.Fatalf("the chain must stop at the repeat, next is %d", got)
	}
}

func TestChainStopsAtItsBound(t *testing.T) {
	root := fakeCheckout(t)
	chainStub(t, handoffSaying("one"), handoffSaying("two"), handoffSaying("three"))
	passthroughSandbox(t)

	code, state := runHere(t, opts{ceiling: 50, calls: 30, sessions: 2,
		checkout: root, args: []string{"fix the four tests"}})
	if code != 1 {
		t.Fatalf("a chain that ran out of sessions must exit 1, got %d", code)
	}
	if got := chain.NextSession(chainDirOf(t, state)); got != 3 {
		t.Fatalf("the bound must be two sessions, next is %d", got)
	}
}

// Starting clean is the default because one handoff per repository was wrong: a second
// instruction in the same checkout would resume the first and then overwrite what it knew.
func TestASecondInstructionStartsItsOwnChain(t *testing.T) {
	root := fakeCheckout(t)
	chainStub(t, handoffSaying("none"), handoffSaying("none"))
	passthroughSandbox(t)
	repo := t.TempDir()
	t.Chdir(repo)
	url := healthy(t, http.StatusOK)

	for _, goal := range []string{"count the rows", "fix the tests"} {
		if _, err := run(opts{ceiling: 50, calls: 30, sessions: 4, checkout: root,
			endpoint: url, noServe: true, args: []string{goal}}); err != nil {
			t.Fatal(err)
		}
	}
	state, err := repoState(repo)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := chain.Chains(state)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("two instructions are two chains, got %v", ids)
	}
}

// -continue carries on the newest chain, and re-issues its instruction rather than asking
// for it again: a goal retyped at each hop is a goal that drifts.
func TestContinueCarriesOnTheNewestChainAndReIssuesItsGoal(t *testing.T) {
	root := fakeCheckout(t)
	chainStub(t, handoffSaying("fix Clamp"), handoffSaying("none"))
	passthroughSandbox(t)
	repo := t.TempDir()
	t.Chdir(repo)
	url := healthy(t, http.StatusOK)

	if _, err := run(opts{ceiling: 50, calls: 30, sessions: 1, checkout: root,
		endpoint: url, noServe: true, args: []string{"fix the four tests"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := run(opts{ceiling: 50, calls: 30, sessions: 4, checkout: root,
		endpoint: url, noServe: true, cont: true}); err != nil {
		t.Fatal(err)
	}
	state, err := repoState(repo)
	if err != nil {
		t.Fatal(err)
	}
	dir := chainDirOf(t, state)
	if got := chain.NextSession(dir); got != 3 {
		t.Fatalf("the second run must add to the same chain, next is %d", got)
	}
	if got := strings.TrimSpace(string(chain.Read(filepath.Join(dir, "goal.txt")))); got != "fix the four tests" {
		t.Fatalf("the chain must re-issue its own instruction, got %q", got)
	}
}

// A fork is a second attempt from the same knowledge: it takes the handoff and the goal
// and leaves the sessions behind.
func TestForkTakesWhatAChainKnewAndNoneOfItsSessions(t *testing.T) {
	root := fakeCheckout(t)
	chainStub(t, handoffSaying("fix Clamp"), handoffSaying("none"))
	passthroughSandbox(t)
	repo := t.TempDir()
	t.Chdir(repo)
	url := healthy(t, http.StatusOK)

	if _, err := run(opts{ceiling: 50, calls: 30, sessions: 1, checkout: root,
		endpoint: url, noServe: true, args: []string{"fix the four tests"}}); err != nil {
		t.Fatal(err)
	}
	state, err := repoState(repo)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := chain.Chains(state)
	if err != nil || len(ids) != 1 {
		t.Fatalf("chains: %v %v", ids, err)
	}
	if _, err := run(opts{ceiling: 50, calls: 30, sessions: 4, checkout: root,
		endpoint: url, noServe: true, fork: ids[0]}); err != nil {
		t.Fatal(err)
	}
	after, err := chain.Chains(state)
	if err != nil || len(after) != 2 {
		t.Fatalf("a fork is a new chain: %v %v", after, err)
	}
	forked := filepath.Join(state, "chains", after[0])
	if !strings.Contains(string(chain.Read(chain.LatestHandoff(forked))), "Next:") {
		t.Fatal("a fork must start from what the chain it forked knew")
	}
	if got := strings.TrimSpace(string(chain.Read(filepath.Join(forked, "goal.txt")))); got != "fix the four tests" {
		t.Fatalf("a fork must carry the goal too, got %q", got)
	}
}

func TestResumeRefusesAChainThatIsNotHere(t *testing.T) {
	root := fakeCheckout(t)
	chainStub(t, handoffSaying("none"))
	passthroughSandbox(t)
	t.Chdir(t.TempDir())
	code, err := run(opts{ceiling: 50, calls: 30, sessions: 4, checkout: root,
		endpoint: healthy(t, http.StatusOK), noServe: true, resume: "20200101-000000"})
	if code != 2 || err == nil {
		t.Fatalf("an unknown chain must be refused: code %d err %v", code, err)
	}
}

// Carrying on a finished chain would start a session whose whole inheritance is `Next:
// none`: it does nothing and writes another one. The refusal names the two things that are
// not nothing.
func TestContinueRefusesAChainThatSaidItWasFinished(t *testing.T) {
	root := fakeCheckout(t)
	chainStub(t, handoffSaying("none"))
	passthroughSandbox(t)
	repo := t.TempDir()
	t.Chdir(repo)
	url := healthy(t, http.StatusOK)

	if _, err := run(opts{ceiling: 50, calls: 30, sessions: 4, checkout: root,
		endpoint: url, noServe: true, args: []string{"fix the four tests"}}); err != nil {
		t.Fatal(err)
	}
	code, err := run(opts{ceiling: 50, calls: 30, sessions: 4, checkout: root,
		endpoint: url, noServe: true, cont: true})
	if code != 2 || err == nil {
		t.Fatalf("a finished chain must be refused, got code %d err %v", code, err)
	}
	if !strings.Contains(err.Error(), "-fork") {
		t.Fatalf("the refusal must name the way on: %v", err)
	}
}

// None of the other bounds is a clock. A denied call still costs a turn, and a turn at
// depth is minutes, so a session that answers a spent budget by trying another tool would
// run until its calls ran out rather than until it had anything to say.
func TestASessionThatWillNotStopIsStoppedAndEndsTheChain(t *testing.T) {
	root := fakeCheckout(t)
	stubClaude(t, "sleep 30")
	passthroughSandbox(t)
	repo := t.TempDir()
	t.Chdir(repo)

	code, err := run(opts{ceiling: 50, calls: 30, sessions: 4, timeout: 200 * time.Millisecond,
		checkout: root, endpoint: healthy(t, http.StatusOK), noServe: true,
		args: []string{"fix the four tests"}})
	if err != nil || code != 1 {
		t.Fatalf("a chain whose session ran past the clock must stop: code %d err %v", code, err)
	}
	state, err := repoState(repo)
	if err != nil {
		t.Fatal(err)
	}
	// One session, not four: a chain does not spend its bound on a session that hangs.
	if got := chain.NextSession(chainDirOf(t, state)); got != 2 {
		t.Fatalf("the chain must stop at the first timeout, next is %d", got)
	}
}

// A session that wrote no handoff must not erase the chain's memory: the next one inherits
// the newest handoff the chain holds, which is the one before it.
func TestASilentSessionDoesNotCostTheChainWhatItKnew(t *testing.T) {
	root := fakeCheckout(t)
	// Second session writes nothing at all; third writes a handoff again.
	chainStub(t, handoffSaying("fix Clamp"), "", handoffSaying("none"))
	passthroughSandbox(t)

	code, state := runHere(t, opts{checkout: root, args: []string{"fix the four tests"}})
	if code != 0 {
		t.Fatalf("the chain must still finish, got %d", code)
	}
	dir := chainDirOf(t, state)
	if got := chain.LatestHandoff(dir); got != filepath.Join(dir, "03", chain.HandoffName) {
		t.Fatalf("latest handoff: got %s", got)
	}
}
