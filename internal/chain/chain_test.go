package chain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The property the whole gate rests on: whatever it permits, the session still has room
// for the result of that call, for the turn that asked for it, and for the turn that
// writes the handoff after the denial. A ceiling that does not leave those is a session
// that hits the wall while being protected from it.
func TestCeilingLeavesRoomForTheHandoffAfterIt(t *testing.T) {
	for _, w := range []struct{ maxContext, maxOutput int }{
		{45056, 4096}, {32768, 4096}, {24576, 2048}, {16384, 1024},
	} {
		l, err := NewLimits(w.maxContext, w.maxOutput, 100, 30)
		if err != nil {
			t.Fatalf("%d/%d: %v", w.maxContext, w.maxOutput, err)
		}
		spent := l.Ceiling + l.Batch*(l.ResultCap/bytesPerToken) + 2*w.maxOutput
		if spent > l.Window {
			t.Fatalf("%d/%d: a session permitted at %d needs %d of a %d window",
				w.maxContext, w.maxOutput, l.Ceiling, spent, l.Window)
		}
	}
}

// A window too small to work in is refused rather than clamped: the session that discovers
// it instead pays a cold ingest to say `Prompt is too long`, having done nothing.
func TestNewLimitsRefusesAWindowNothingFitsIn(t *testing.T) {
	if _, err := NewLimits(8192, 4096, 50, 30); err == nil {
		t.Fatal("8,192 against a 4,096 reservation leaves less than the preamble and must be refused")
	}
	if _, err := NewLimits(45056, 4096, 50, 30); err != nil {
		t.Fatalf("the shipped window must be workable: %v", err)
	}
}

// The requested fraction is what binds while it is the smaller of the two, and the derived
// headroom is what binds when it is not. Both have to, or the flag either does nothing or
// can be set to something unsafe.
func TestCeilingIsTheSmallerOfWhatWasAskedForAndWhatIsSafe(t *testing.T) {
	half, err := NewLimits(45056, 4096, 40, 30)
	if err != nil {
		t.Fatal(err)
	}
	if want := 40960 * 40 / 100; half.Ceiling != want {
		t.Fatalf("40%% of a 40,960 window is %d, got %d", want, half.Ceiling)
	}
	all, err := NewLimits(45056, 4096, 100, 30)
	if err != nil {
		t.Fatal(err)
	}
	if all.Ceiling >= all.Window {
		t.Fatalf("100%% must still be held under the window by the reserve, got %d of %d",
			all.Ceiling, all.Window)
	}
}

// The point of the feature: one constant fitted neither end of the range served, so the
// budget follows the ceiling the same way every other bound here does.
func TestTheCallBudgetFollowsTheCeiling(t *testing.T) {
	small, err := NewLimits(32768, 4096, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	large, err := NewLimits(49152, 4096, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if small.Calls >= large.Calls {
		t.Fatalf("a %d ceiling must buy fewer calls than a %d one, got %d and %d",
			small.Ceiling, large.Ceiling, small.Calls, large.Calls)
	}
	// The room above the preamble, and nothing else: a budget that counted the preamble
	// would spend it on the context every session pays before it has done anything.
	if want := (large.Ceiling - preambleFloor) / callCost; large.Calls != want {
		t.Fatalf("a %d ceiling has room for %d calls, got %d", large.Ceiling, want, large.Calls)
	}
}

// An explicit number is what makes a measurement repeatable, so it is taken as given —
// including below the floor the derived one is held to.
func TestAnExplicitCallBudgetOverridesTheDerivedOne(t *testing.T) {
	l, err := NewLimits(45056, 4096, 100, 3)
	if err != nil {
		t.Fatalf("a named budget must be taken as given: %v", err)
	}
	if l.Calls != 3 {
		t.Fatalf("a named budget of 3 must survive, got %d", l.Calls)
	}
	if _, err := NewLimits(45056, 4096, 100, -1); err == nil {
		t.Fatal("a negative budget runs nothing and must be refused")
	}
}

// A ceiling with room for less than one turn's calls is a configuration mistake, and the
// same argument the preamble floor already makes applies: refuse it rather than start a
// session that cannot read a file, change it and see what that did.
func TestNewLimitsRefusesACeilingWithNoRoomToWorkIn(t *testing.T) {
	// 12% of the shipped window clears the preamble floor and little else, which is the
	// band this refusal exists for: the old one passed it.
	_, err := NewLimits(45056, 4096, 12, 0)
	if err == nil {
		t.Fatal("a ceiling with room for one call must be refused")
	}
	if !strings.Contains(err.Error(), "tool calls") {
		t.Fatalf("the refusal must name what ran out: %v", err)
	}
}

const handoffPath = "/state/01/HANDOFF.md"

func limits(t *testing.T) Spec {
	t.Helper()
	l, err := NewLimits(24576, 1024, 100, 3)
	if err != nil {
		t.Fatal(err)
	}
	return Spec{Limits: l, Handoff: handoffPath}
}

func work() Payload {
	return Payload{ToolName: "Bash", ToolInput: map[string]any{"command": "go test ./..."}}
}

func handoffCall() Payload {
	return Payload{ToolName: "Write", ToolInput: map[string]any{"file_path": handoffPath}}
}

// The way out is never part of the work, so a spent budget must not close it. This is the
// difference between a session that hands over and one that dies holding what it learned.
func TestGatePermitsTheHandoffAfterTheBudgetIsSpent(t *testing.T) {
	spec := limits(t)
	l := spec.Limits
	spent := State{Calls: l.Calls, Peak: l.Ceiling + 1}
	if v := Gate(work(), spec, spent); !v.Deny {
		t.Fatal("work must be denied once the budget is spent")
	}
	if v := Gate(handoffCall(), spec, spent); v.Deny {
		t.Fatalf("the handoff must stay permitted: %s", v.Reason)
	}
}

func TestGateDeniesOnTheCeilingAndOnTheCallBudget(t *testing.T) {
	spec := limits(t)
	l := spec.Limits
	byContext := Gate(work(), spec, State{Calls: 0, Peak: l.Ceiling})
	if !byContext.Deny || !strings.Contains(byContext.Reason, "ceiling") {
		t.Fatalf("the context ceiling must deny and say so: %+v", byContext)
	}
	byCalls := Gate(work(), spec, State{Calls: l.Calls, Peak: 10})
	if !byCalls.Deny || !strings.Contains(byCalls.Reason, "tool calls") {
		t.Fatalf("the call budget must deny and say so: %+v", byCalls)
	}
}

// A transcript that cannot be read yields -1, which must not read as "plenty of room". The
// call budget is what bounds a session whose context the gate cannot see.
func TestGateStillBoundsASessionItCannotMeasure(t *testing.T) {
	spec := limits(t)
	l := spec.Limits
	if v := Gate(work(), spec, State{Calls: 0, Peak: -1}); v.Deny {
		t.Fatalf("an unmeasurable first call must be permitted: %s", v.Reason)
	}
	if v := Gate(work(), spec, State{Calls: l.Calls, Peak: -1}); !v.Deny {
		t.Fatal("an unmeasurable session must still be stopped by its call budget")
	}
}

// The harness issues a turn's calls together and the transcript does not change while they
// run, so one reading decides all of them. Measured without this bound: a five-call turn
// carried the context 1,960 tokens past a ceiling it had been under when the gate looked.
func TestTurnBoundsItsCallsSoOneReadingCannotDecideAnyNumber(t *testing.T) {
	l := limits(t).Limits
	if v := Turn(l, l.Batch-1); v.Deny {
		t.Fatalf("a turn under its bound must be permitted: %s", v.Reason)
	}
	v := Turn(l, l.Batch)
	if !v.Deny {
		t.Fatal("a turn past its bound must be refused")
	}
	// A throttle, not an end: saying otherwise would tell a session with most of its window
	// left to hand over.
	if strings.Contains(v.Reason, "other than writing the handoff") {
		t.Fatalf("a turn's bound is not the session's: %q", v.Reason)
	}
}

// Permitting the handoff without bounding it turns a session that cannot write one into a
// session that never ends.
func TestGateBoundsTheHandoffItself(t *testing.T) {
	spec := limits(t)
	if v := Gate(handoffCall(), spec, State{Handoffs: handoffGrace}); !v.Deny {
		t.Fatal("a session rewriting its handoff forever must be stopped")
	}
}

// The denial is the model's only account of why its tools stopped working, and it reaches
// it verbatim. It states the state; the protocol is in the system prompt, because an
// instruction arriving through a tool result is refused as injection and should be.
func TestDenialStatesTheStateAndGivesNoInstruction(t *testing.T) {
	spec := limits(t)
	l := spec.Limits
	reason := Gate(work(), spec, State{Calls: l.Calls, Peak: 0}).Reason
	for _, told := range []string{"Write ", "you must", "now —", "then stop"} {
		if strings.Contains(reason, told) {
			t.Fatalf("the denial instructs rather than reports: %q", reason)
		}
	}
	if !strings.Contains(reason, "localcode:") {
		t.Fatalf("the denial must say who refused: %q", reason)
	}
}

const good = "# Handoff\n\n**Box:** the third one\n**Files:** `median.go`\n" +
	"**Tried:** fixed the even-length case, TestMedianEven passes\n**Next:** fix Clamp\n"

func TestStopRefusesWithoutAHandoffAndRelentsRatherThanWedging(t *testing.T) {
	if v := Stop(nil, "/s/HANDOFF.md", 0); !v.Deny {
		t.Fatal("a session that wrote no handoff must not be allowed to end")
	}
	if v := Stop([]byte("# Handoff\n"), "/s/HANDOFF.md", 0); !v.Deny {
		t.Fatal("a heading is not a handoff")
	}
	if v := Stop([]byte(good), "/s/HANDOFF.md", 0); v.Deny {
		t.Fatalf("a usable handoff must let the session end: %s", v.Reason)
	}
	// The bound is what makes this safe to run unattended: the extractor is the floor.
	if v := Stop(nil, "/s/HANDOFF.md", stopTries); v.Deny {
		t.Fatal("after its refusals the hook must relent rather than wedge the run")
	}
}

// Seven of eight handoffs in the measured chain carried `Prompt is too long` as their next
// step. What is read back out of one is therefore the line, not the file.
func TestNextAndDoneReadTheChainsOnlySignals(t *testing.T) {
	if got := Next([]byte(good)); got != "fix Clamp" {
		t.Fatalf("Next: got %q", got)
	}
	if Done([]byte(good)) {
		t.Fatal("a chain with work left is not done")
	}
	if !Done([]byte(strings.Replace(good, "fix Clamp", "none", 1))) {
		t.Fatal("`none` is the completion signal the system prompt names")
	}
	if Next(nil) != "" {
		t.Fatal("a handoff with no Next line has none")
	}
}

// The hook that lets a session end and the supervisor that reads what it left must mean
// the same thing by a handoff. They did not: the hook matched the marker as a substring
// and the supervisor read the step after it, so two sessions were told they had handed
// something on while the chain read nothing in either.
func TestTheStopHookAndTheSupervisorAgreeOnAUsableHandoff(t *testing.T) {
	bare := []byte(strings.Replace(good, "**Next:** fix Clamp", "**Next:**", 1))
	if len(bare) < handoffFloor {
		t.Fatalf("the case must clear the size floor to test the other half: %d bytes", len(bare))
	}
	if v := Stop(bare, "/s/HANDOFF.md", 0); !v.Deny {
		t.Fatal("a marker carrying no step hands nothing on, whatever the file's size")
	}
	// What the hook lets end is what the supervisor can read, in both directions.
	for _, h := range []string{good, strings.Replace(good, "fix Clamp", "\n1. fix Clamp", 1)} {
		ended := !Stop([]byte(h), "/s/HANDOFF.md", 0).Deny
		if read := Next([]byte(h)) != ""; ended != read {
			t.Fatalf("hook let it end: %v, supervisor read a step: %v, for %q", ended, read, h)
		}
	}
}

// A marker written as a heading over a list is still a next step. Measured on a chain
// that stopped at seven of eight sessions: two handoffs read as empty, and the repeat
// guard took the two empties for a plan written twice.
func TestNextReadsTheStepWrittenBelowTheMarker(t *testing.T) {
	block := "# Handoff\n\n**Box:** the third one\n**Next:**\n1. fix Clamp\n2. then ship it\n"
	if got := Next([]byte(block)); got != "1. fix Clamp 2. then ship it" {
		t.Fatalf("Next: got %q", got)
	}
	spaced := "**Next:**\n\n  fix Clamp\n"
	if got := Next([]byte(spaced)); got != "fix Clamp" {
		t.Fatalf("a marker spaced like a heading still carries its step: got %q", got)
	}
	closing := "**Next:**\nfix Clamp\n\nthe worktree is left in place.\n"
	if got := Next([]byte(closing)); got != "fix Clamp" {
		t.Fatalf("the step ends at the blank line, not at the file: got %q", got)
	}
	fields := "**Next:**\nfix Clamp\n**Files:** `median.go`\n"
	if got := Next([]byte(fields)); got != "fix Clamp" {
		t.Fatalf("the step ends at the next field: got %q", got)
	}
	if got := Next([]byte("**Next:**\n")); got != "" {
		t.Fatalf("a marker with nothing under it carries no step: got %q", got)
	}
	// The completion signal is read out of the same place, so it is unreadable for the
	// same reason until this is.
	if !Done([]byte("**Next:**\nnone \u2014 every box is ticked and the branch is pushed.\n")) {
		t.Fatal("`none` under the marker is the same completion signal as `none` on it")
	}
}

// A spent budget opens exactly one door, and it is the door the supervisor reads from.
func TestOnlyTheOneHandoffTheSessionWasGivenCounts(t *testing.T) {
	if !IsHandoff("Write", map[string]any{"file_path": "/state/01/./HANDOFF.md"}, handoffPath) {
		t.Fatal("the path the session was given is the handoff")
	}
	if IsHandoff("Write", map[string]any{"file_path": "/repo/HANDOFF.md"}, handoffPath) {
		t.Fatal("a file named like the handoff is not the handoff")
	}
	if IsHandoff("Write", map[string]any{"file_path": "/repo/median.go"}, handoffPath) {
		t.Fatal("work is not the handoff")
	}
	if IsHandoff("Bash", map[string]any{"command": "echo > HANDOFF.md"}, handoffPath) {
		t.Fatal("only the file tools write the handoff; a shell can write anything")
	}
}

func lineFile(t *testing.T, lines int) string {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= lines; i++ {
		fmt.Fprintf(&b, "line %d: the quick brown fox jumps over the lazy dog\n", i)
	}
	path := filepath.Join(t.TempDir(), "big.txt")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// `Bash` takes its cap from the harness and `Read` has none: its own bound is two thousand
// lines, which bounds lines rather than the window. The clamp comes from the file, because
// how many of its lines fit in the reserve is a question the file answers and a
// tokens-per-line guess does not.
func TestClampReadHoldsALongFileToTheReserve(t *testing.T) {
	path := lineFile(t, 500) // ~25 KB
	got, clamped := ClampRead(map[string]any{"file_path": path}, 1000)
	if !clamped {
		t.Fatal("a 25 KB file against a 1 KB reserve must be clamped")
	}
	lines, _ := got["limit"].(int)
	if lines < 15 || lines > 21 {
		t.Fatalf("about a thousand bytes of 50-byte lines is around twenty, got %d", lines)
	}
	if got["file_path"] != path {
		t.Fatal("the rest of the call must survive the clamp")
	}
}

// Most reads are of ordinary files and must go through untouched, or the gate is spending
// the session's window on its behalf.
func TestClampReadLeavesAReadThatAlreadyFits(t *testing.T) {
	if _, clamped := ClampRead(map[string]any{"file_path": lineFile(t, 5)}, 1000); clamped {
		t.Fatal("a file inside the reserve must not be clamped")
	}
	if _, clamped := ClampRead(map[string]any{"file_path": "/no/such/file"}, 1000); clamped {
		t.Fatal("an unreadable file is the tool's error to report, not the gate's to guess at")
	}
	if _, clamped := ClampRead(map[string]any{}, 1000); clamped {
		t.Fatal("a call with no path is not a read of anything")
	}
}

// A session that asked for ten lines wanted ten. Widening it would spend the window for it.
func TestClampReadNeverWidensWhatWasAskedFor(t *testing.T) {
	path := lineFile(t, 500)
	// json numbers arrive as float64, which is how the payload really reaches this.
	if _, clamped := ClampRead(map[string]any{"file_path": path, "limit": float64(5)}, 1000); clamped {
		t.Fatal("a limit under the reserve is the one that binds")
	}
	got, clamped := ClampRead(map[string]any{"file_path": path, "limit": float64(400)}, 1000)
	if !clamped {
		t.Fatal("a limit over the reserve must still be brought down")
	}
	if lines, _ := got["limit"].(int); lines >= 400 {
		t.Fatalf("got %d", lines)
	}
}

// Counting from an offset is what a session does when it comes back for more of a file.
func TestClampReadCountsFromWhereTheReadStarts(t *testing.T) {
	path := lineFile(t, 500)
	// Ten lines from the end are about 500 bytes and fit; the whole file does not.
	if _, clamped := ClampRead(map[string]any{"file_path": path, "offset": float64(491)}, 1000); clamped {
		t.Fatal("what is left after the offset is what has to fit, not the whole file")
	}
	if _, clamped := ClampRead(map[string]any{"file_path": path}, 1000); !clamped {
		t.Fatal("the whole file does not fit and must still be clamped")
	}
}

// Every one of these is a real `**Next:**` line, taken from chains run against the model.
// The first four ended a chain and the rest must not: "none of the tests pass" is the
// opposite of done and begins the same way.
func TestDoneReadsTheFirstClauseAndNotTheWholeLine(t *testing.T) {
	finished := []string{
		"none",
		"none — 0003 is done. Remaining: human merges 0001 and 0003 branches, then 0002 unblocks",
		"none for an agent. Human merges the two branches (0001 first — 0002 needs it)",
		"nothing left to do",
		"done.",
		"no further work",
	}
	for _, line := range finished {
		if !Done([]byte("**Next:** " + line + "\n")) {
			t.Fatalf("this ends a chain and did not: %q", line)
		}
	}
	unfinished := []string{
		"none of the tests pass — fix Clamp first",
		"nothing works yet, start with median.go",
		"no tests exist for the even-length case",
		"fix Clamp",
		"",
	}
	for _, line := range unfinished {
		if Done([]byte("**Next:** " + line + "\n")) {
			t.Fatalf("this is not done and was read as done: %q", line)
		}
	}
	// A handoff with no Next line at all is not a finished one.
	if Done([]byte("# Handoff\n")) {
		t.Fatal("a handoff carrying no Next is not done")
	}
}
