package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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
func chainStub(t *testing.T, handoffs ...string) string {
	t.Helper()
	dir := t.TempDir()
	count := filepath.Join(dir, "invocations")
	var cases strings.Builder
	for i, h := range handoffs {
		fmt.Fprintf(&cases, "%d)\ncat > \"$out\" <<'HANDOFF_EOF'\n%s\nHANDOFF_EOF\n;;\n", i+1, h)
	}
	body := "#!/bin/sh\n" +
		"n=$(cat '" + count + "' 2>/dev/null || echo 0); n=$((n+1)); echo $n > '" + count + "'\n" +
		"printf '%s\\n' \"$LOCALCODE_INHERIT\" > '" + dir + "'/inherit-$n\n" +
		"printf '%s\\n' \"$*\" > '" + dir + "'/argv-$n\n" +
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
	return dir
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
	quiet(t)
	repo := t.TempDir()
	t.Chdir(repo)
	o.endpoint, o.noServe = healthy(t, http.StatusOK), true
	if o.ceiling == 0 {
		o.ceiling, o.calls, o.sessions = 100, 30, 8
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

// quiet keeps a chain's narration out of the test log. What each session did is asserted,
// so printing it as well only hides whichever test actually failed.
func quiet(t *testing.T) {
	t.Helper()
	old := progressOut
	progressOut = io.Discard
	t.Cleanup(func() { progressOut = old })
}

func chainDirOf(t *testing.T, state string) string {
	t.Helper()
	ids, err := chain.Chains(state)
	if err != nil || len(ids) != 1 {
		t.Fatalf("chains: %v %v", ids, err)
	}
	return filepath.Join(state, "chains", ids[0])
}

// endingOf reads back how a chain said it stopped, which is the record a background run
// leaves behind once its narration has gone to a stream nobody kept.
func endingOf(t *testing.T, dir string) chain.Ending {
	t.Helper()
	e, ok := chain.ReadEnding(dir)
	if !ok {
		t.Fatalf("the chain recorded no ending in %s", filepath.Join(dir, chain.EndingName))
	}
	// Only when it names one: a chain whose sessions wrote nothing has no handoff to send
	// a reader to, and inventing a path would be worse than saying so.
	if e.Handoff != "" {
		if _, err := os.Stat(e.Handoff); err != nil {
			t.Fatalf("the ending points at %s, which a reader cannot open: %v", e.Handoff, err)
		}
	}
	return e
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
	// The narration went to a stream. What a reader has afterwards is this.
	if e := endingOf(t, dir); e.Reason != chain.Finished || e.Session != 3 {
		t.Fatalf("a finished chain must record finishing at session 3, got %+v", e)
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
	dir := chainDirOf(t, state)
	if got := chain.NextSession(dir); got != 3 {
		t.Fatalf("the chain must stop at the repeat, next is %d", got)
	}
	if e := endingOf(t, dir); e.Reason != chain.Stalled || e.Session != 2 {
		t.Fatalf("a stalled chain must record stalling at session 2, got %+v", e)
	}
}

// Two handoffs the supervisor could not read are not one plan written twice. Read as
// that, they stopped a seven-session chain with a session of its bound unspent.
func TestChainCarriesOnWhenItCouldNotReadTwoNextSteps(t *testing.T) {
	root := fakeCheckout(t)
	chainStub(t, handoffSaying(""), handoffSaying(""), handoffSaying("none"))
	passthroughSandbox(t)

	code, state := runHere(t, opts{checkout: root, args: []string{"fix the four tests"}})
	if code != 0 {
		t.Fatalf("an unreadable pair is not a stall, got %d", code)
	}
	if got := chain.NextSession(chainDirOf(t, state)); got != 4 {
		t.Fatalf("the chain must have run all three sessions, next is %d", got)
	}
}

func TestChainStopsAtItsBound(t *testing.T) {
	root := fakeCheckout(t)
	chainStub(t, handoffSaying("one"), handoffSaying("two"), handoffSaying("three"))
	passthroughSandbox(t)

	code, state := runHere(t, opts{ceiling: 100, calls: 30, sessions: 2,
		checkout: root, args: []string{"fix the four tests"}})
	if code != 1 {
		t.Fatalf("a chain that ran out of sessions must exit 1, got %d", code)
	}
	dir := chainDirOf(t, state)
	if got := chain.NextSession(dir); got != 3 {
		t.Fatalf("the bound must be two sessions, next is %d", got)
	}
	if e := endingOf(t, dir); e.Reason != chain.Bound || e.Session != 2 {
		t.Fatalf("a bounded chain must record its bound at session 2, got %+v", e)
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
	quiet(t)
	url := healthy(t, http.StatusOK)

	for _, goal := range []string{"count the rows", "fix the tests"} {
		if _, err := run(opts{ceiling: 100, calls: 30, sessions: 4, checkout: root,
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
	quiet(t)
	url := healthy(t, http.StatusOK)

	if _, err := run(opts{ceiling: 100, calls: 30, sessions: 1, checkout: root,
		endpoint: url, noServe: true, args: []string{"fix the four tests"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := run(opts{ceiling: 100, calls: 30, sessions: 4, checkout: root,
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
	quiet(t)
	url := healthy(t, http.StatusOK)

	if _, err := run(opts{ceiling: 100, calls: 30, sessions: 1, checkout: root,
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
	if _, err := run(opts{ceiling: 100, calls: 30, sessions: 4, checkout: root,
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
	quiet(t)
	code, err := run(opts{ceiling: 100, calls: 30, sessions: 4, checkout: root,
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
	quiet(t)
	url := healthy(t, http.StatusOK)

	if _, err := run(opts{ceiling: 100, calls: 30, sessions: 4, checkout: root,
		endpoint: url, noServe: true, args: []string{"fix the four tests"}}); err != nil {
		t.Fatal(err)
	}
	code, err := run(opts{ceiling: 100, calls: 30, sessions: 4, checkout: root,
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
	quiet(t)

	code, err := run(opts{ceiling: 100, calls: 30, sessions: 4, timeout: 200 * time.Millisecond,
		checkout: root, endpoint: healthy(t, http.StatusOK), noServe: true,
		args: []string{"fix the four tests"}})
	if err != nil || code != 1 {
		t.Fatalf("a chain whose session ran past the clock must stop: code %d err %v", code, err)
	}
	state, err := repoState(repo)
	if err != nil {
		t.Fatal(err)
	}
	dir := chainDirOf(t, state)
	// One session, not four: a chain does not spend its bound on a session that hangs.
	if got := chain.NextSession(dir); got != 2 {
		t.Fatalf("the chain must stop at the first timeout, next is %d", got)
	}
	if e := endingOf(t, dir); e.Reason != chain.TimedOut || e.Session != 1 {
		t.Fatalf("a timed-out chain must record the clock at session 1, got %+v", e)
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

// The developer's own loop: a session with no instruction runs out, hands over, and
// `-continue` picks it up. There is no chain to run it, so the handoff has to reach the
// next session through the SessionStart hook — and that session has to stay interactive,
// because `-p` would answer once and exit.
func TestContinueCarriesAnInteractiveSessionOnWithoutAPrompt(t *testing.T) {
	root := fakeCheckout(t)
	stub := chainStub(t, handoffSaying("fix Clamp"), handoffSaying("fix Title"))
	passthroughSandbox(t)
	repo := t.TempDir()
	t.Chdir(repo)
	quiet(t)
	url := healthy(t, http.StatusOK)

	// No args at all: this is somebody at a keyboard.
	if _, err := run(opts{ceiling: 100, calls: 30, sessions: 8, checkout: root,
		endpoint: url, noServe: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := run(opts{ceiling: 100, calls: 30, sessions: 8, checkout: root,
		endpoint: url, noServe: true, cont: true}); err != nil {
		t.Fatal(err)
	}

	state, err := repoState(repo)
	if err != nil {
		t.Fatal(err)
	}
	dir := chainDirOf(t, state)
	if got := chain.NextSession(dir); got != 3 {
		t.Fatalf("-continue must add to the same chain, next is %d", got)
	}
	// The second session inherits the first session's handoff, not its own empty file.
	inherit := strings.TrimSpace(string(chain.Read(filepath.Join(stub, "inherit-2"))))
	if want := filepath.Join(dir, "01", chain.HandoffName); inherit != want {
		t.Fatalf("the second session inherited %q, want %q", inherit, want)
	}
	if chain.Next(chain.Read(inherit)) != "fix Clamp" {
		t.Fatalf("what it inherited is not the handoff the first session wrote")
	}
	// And it is still a conversation, not a one-shot answer.
	if argv := string(chain.Read(filepath.Join(stub, "argv-2"))); strings.Contains(argv, " -p ") {
		t.Fatalf("an interactive continuation must not be -p: %s", argv)
	}
}

// The listing is the one place a reader looks before deciding whether to resume, so how a
// chain ended belongs in it: `finished` and `bound` are the same session count and
// opposite answers.
func TestSessionsListsHowEachChainEnded(t *testing.T) {
	root := fakeCheckout(t)
	chainStub(t, handoffSaying("fix Clamp"), handoffSaying("fix Title"))
	passthroughSandbox(t)

	_, state := runHere(t, opts{ceiling: 100, calls: 30, sessions: 2,
		checkout: root, args: []string{"fix the four tests"}})

	var out strings.Builder
	if code, err := listChains(state, &out); code != 0 || err != nil {
		t.Fatalf("listing a chain: code %d err %v", code, err)
	}
	line := strings.TrimSpace(out.String())
	if !strings.Contains(line, "2 sessions") || !strings.Contains(line, string(chain.Bound)) {
		t.Fatalf("a bounded chain must list its bound beside its session count, got %q", line)
	}
}

// A chain that ran before the ending was recorded still lists an honest word, and the one
// its handoff can answer for is `finished`.
func TestSessionsReadsAnOlderChainsEndingFromItsHandoff(t *testing.T) {
	root := fakeCheckout(t)
	chainStub(t, handoffSaying("none"))
	passthroughSandbox(t)

	_, state := runHere(t, opts{checkout: root, args: []string{"fix the four tests"}})
	chain.ClearEnding(chainDirOf(t, state))

	var out strings.Builder
	if _, err := listChains(state, &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, string(chain.Finished)) {
		t.Fatalf("a finished chain with no record must still read finished, got %q", got)
	}
}

// gitRepo is a repository a chain can be judged against, since a plain directory is one
// this cannot read.
func gitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"}, {"add", "."}, {"commit", "-qm", "first", "--allow-empty"},
	} {
		full := append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t",
			"-c", "commit.gpgsign=false"}, args...)
		if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// The row is where a supervisor reads a session back, and what the last one changed is not
// recoverable from a handoff the model wrote about itself.
func TestASessionsRowSaysWhetherTheRepositoryMoved(t *testing.T) {
	root := fakeCheckout(t)
	// The first session writes a file into the repository it was given; the second only
	// writes its handoff, which lives outside it.
	stub := chainStub(t, handoffSaying("fix Clamp"), handoffSaying("none"))
	// The stub ends in `exit 0`, so the edit goes in ahead of the case that writes the
	// handoff rather than after it.
	body := strings.Replace(string(mustRead(t, filepath.Join(stub, "claude"))),
		"out=/dev/null\n", "[ \"$n\" = 1 ] && : > \"$PWD/fixed.go\"\nout=/dev/null\n", 1)
	if err := os.WriteFile(filepath.Join(stub, "claude"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	passthroughSandbox(t)
	quiet(t)
	repo := t.TempDir()
	t.Chdir(repo)
	gitRepo(t, repo)

	if _, err := run(opts{ceiling: 100, calls: 30, sessions: 4, checkout: root,
		endpoint: healthy(t, http.StatusOK), noServe: true,
		args: []string{"fix the four tests"}}); err != nil {
		t.Fatal(err)
	}
	state, err := repoState(repo)
	if err != nil {
		t.Fatal(err)
	}
	rows := sessionRows(t, chainDirOf(t, state))
	if len(rows) != 2 {
		t.Fatalf("two sessions, got %d", len(rows))
	}
	if rows[0].Moved == nil || !*rows[0].Moved {
		t.Fatalf("the session that wrote a file must report the repository moved: %v", rows[0].Moved)
	}
	if rows[1].Moved == nil || *rows[1].Moved {
		t.Fatalf("the session that wrote only its handoff must report it unchanged: %v", rows[1].Moved)
	}
}

// A chain outside version control reports nothing rather than reporting stillness, which
// is a claim it has no way to make.
func TestASessionOutsideARepositoryReportsNoMovement(t *testing.T) {
	root := fakeCheckout(t)
	chainStub(t, handoffSaying("none"))
	passthroughSandbox(t)

	_, state := runHere(t, opts{checkout: root, args: []string{"fix the four tests"}})
	rows := sessionRows(t, chainDirOf(t, state))
	if rows[0].Moved != nil {
		t.Fatalf("no repository means no reading, got %v", *rows[0].Moved)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func sessionRows(t *testing.T, dir string) []row {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, "sessions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []row
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		var r row
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
		rows = append(rows, r)
	}
	return rows
}

// movingStub is a claude that edits the repository on the sessions named and only writes
// its handoff on the rest, with a different next step every time — which is the shape the
// `Next` comparison cannot see through.
func movingStub(t *testing.T, moves map[int]bool, nexts ...string) {
	t.Helper()
	var handoffs []string
	for _, n := range nexts {
		handoffs = append(handoffs, handoffSaying(n))
	}
	stub := chainStub(t, handoffs...)
	var cases string
	for n := range moves {
		cases += fmt.Sprintf("[ \"$n\" = %d ] && echo %d >> \"$PWD/worked.go\"\n", n, n)
	}
	body := strings.Replace(string(mustRead(t, filepath.Join(stub, "claude"))),
		"out=/dev/null\n", cases+"out=/dev/null\n", 1)
	if err := os.WriteFile(filepath.Join(stub, "claude"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// inRepo runs a chain in a fresh git repository and returns the exit code and state dir.
func inRepo(t *testing.T, o opts) (int, string) {
	t.Helper()
	quiet(t)
	repo := t.TempDir()
	t.Chdir(repo)
	gitRepo(t, repo)
	o.endpoint, o.noServe = healthy(t, http.StatusOK), true
	if o.ceiling == 0 {
		o.ceiling, o.calls, o.sessions = 100, 30, 8
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

// The failure this feature exists for: eight sessions in a row committed nothing while the
// `Next` comparison stayed silent, because each restated the same plan differently.
func TestAChainThatStopsChangingTheRepositoryStops(t *testing.T) {
	root := fakeCheckout(t)
	// Session 1 works; 2 and 3 do not, and each plans something new.
	movingStub(t, map[int]bool{1: true},
		"fix Clamp", "fix Title", "fix Median", "fix Mean")
	passthroughSandbox(t)

	code, state := inRepo(t, opts{checkout: root, args: []string{"fix the four tests"}})
	if code != 1 {
		t.Fatalf("a chain that stopped changing anything must exit 1, got %d", code)
	}
	dir := chainDirOf(t, state)
	if got := chain.NextSession(dir); got != 4 {
		t.Fatalf("the chain must stop at the second still session, next is %d", got)
	}
	if e := endingOf(t, dir); e.Reason != chain.Stalled {
		t.Fatalf("it stalled, got %+v", e)
	}
}

// Two, not one. A session that spends its budget reading before it edits is normal, and
// stopping on the first of those would end chains doing real work.
func TestOneStillSessionDoesNotStopAChain(t *testing.T) {
	root := fakeCheckout(t)
	// Session 2 changes nothing; 3 works again and finishes.
	movingStub(t, map[int]bool{1: true, 3: true},
		"fix Clamp", "fix Title", "none")
	passthroughSandbox(t)

	code, state := inRepo(t, opts{checkout: root, args: []string{"fix the four tests"}})
	if code != 0 {
		t.Fatalf("one still session is not a stall, got %d", code)
	}
	if got := chain.NextSession(chainDirOf(t, state)); got != 4 {
		t.Fatalf("all three sessions must run, next is %d", got)
	}
}

// A chain whose work is a question rather than an edit never moves a repository, and
// would be stopped on its second session by a test that counted from the start. Prose is
// the only evidence such a chain has, so movement judges a chain only once it has moved.
func TestAChainThatNeverMovesTheRepositoryIsJudgedOnItsHandoffs(t *testing.T) {
	root := fakeCheckout(t)
	movingStub(t, map[int]bool{},
		"count the rows", "read the log", "check the index", "none")
	passthroughSandbox(t)

	code, state := inRepo(t, opts{checkout: root, args: []string{"find out why it is slow"}})
	if code != 0 {
		t.Fatalf("an investigation must be allowed to finish, got %d", code)
	}
	if got := chain.NextSession(chainDirOf(t, state)); got != 5 {
		t.Fatalf("all four sessions must run, next is %d", got)
	}
}

// loud captures the supervisor's own lines instead of dropping them, for the tests that
// are about what it said.
func loud(t *testing.T) *strings.Builder {
	t.Helper()
	var said strings.Builder
	old := progressOut
	progressOut = &said
	t.Cleanup(func() { progressOut = old })
	return &said
}

// The failure this feature exists for: a chain resumed against a different server took a
// 10,240 ceiling where its sessions before had 22,528, and ran two starved sessions before
// anybody read session.json by hand.
func TestAResumeOnADifferentBudgetSaysSoBeforeItRuns(t *testing.T) {
	root := fakeCheckout(t)
	chainStub(t, handoffSaying("fix Clamp"), handoffSaying("none"))
	passthroughSandbox(t)
	repo := t.TempDir()
	t.Chdir(repo)
	quiet(t)
	url := healthy(t, http.StatusOK)

	if _, err := run(opts{ceiling: 100, calls: 30, sessions: 1, checkout: root,
		endpoint: url, noServe: true, args: []string{"fix the four tests"}}); err != nil {
		t.Fatal(err)
	}
	said := loud(t)
	if _, err := run(opts{ceiling: 50, calls: 30, sessions: 4, checkout: root,
		endpoint: url, noServe: true, cont: true}); err != nil {
		t.Fatal(err)
	}
	got := said.String()
	if !strings.Contains(got, "different budget") {
		t.Fatalf("a resume on a changed budget must say so: %q", got)
	}
	// Both numbers and the endpoint, because a warning that does not name them cannot be
	// acted on without reading session.json anyway.
	for _, want := range []string{"ceiling 22528 -> 20480", url} {
		if !strings.Contains(got, want) {
			t.Fatalf("the warning must name %q: %q", want, got)
		}
	}
}

// A chain carried on with the budget it had says nothing. A warning that fires on every
// resume is one nobody reads.
func TestAResumeOnTheSameBudgetSaysNothing(t *testing.T) {
	root := fakeCheckout(t)
	chainStub(t, handoffSaying("fix Clamp"), handoffSaying("none"))
	passthroughSandbox(t)
	repo := t.TempDir()
	t.Chdir(repo)
	quiet(t)
	url := healthy(t, http.StatusOK)

	if _, err := run(opts{ceiling: 100, calls: 30, sessions: 1, checkout: root,
		endpoint: url, noServe: true, args: []string{"fix the four tests"}}); err != nil {
		t.Fatal(err)
	}
	said := loud(t)
	// A different session bound, which is what a resume is for and not a budget at all.
	if _, err := run(opts{ceiling: 100, calls: 30, sessions: 9, checkout: root,
		endpoint: url, noServe: true, cont: true}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(said.String(), "different budget") {
		t.Fatalf("an unchanged budget must be silent: %q", said.String())
	}
}
