package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// piSession writes a session file the way pi leaves one: an entry per message, a usage on
// each assistant entry, and the entry's own clock.
func piSession(t *testing.T, dir string, rows ...string) string {
	t.Helper()
	path := filepath.Join(dir, "2026-08-25T13-08-22-205Z_01a03909.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assistant(at string, input, cacheRead, output int) string {
	return `{"type":"message","timestamp":"` + at + `","message":{"role":"assistant","usage":{` +
		`"input":` + itoa(input) + `,"output":` + itoa(output) + `,"cacheRead":` + itoa(cacheRead) +
		`,"cacheWrite":0}}}`
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for ; n > 0; n /= 10 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
	}
	return string(digits)
}

// What a chain's account is read off: every call, what it ingested, what the server already
// held, and how long it took. A session's cost is in its own record and in nothing else.
func TestPiRecorderReadsWhatEachCallCost(t *testing.T) {
	dir := t.TempDir()
	path := piSession(t,
		dir,
		`{"type":"session","version":3,"id":"x","timestamp":"2026-08-25T13:08:20Z"}`,
		`{"type":"message","timestamp":"2026-08-25T13:08:21Z","message":{"role":"user"}}`,
		assistant("2026-08-25T13:08:31Z", 2157, 0, 35),
		`{"type":"message","timestamp":"2026-08-25T13:08:32Z","message":{"role":"toolResult"}}`,
		assistant("2026-08-25T13:08:52Z", 24, 2191, 25),
	)

	var rec piRecorder
	if got := rec.Transcript(dir); got != path {
		t.Fatalf("the session file is the one in the directory it was told to write: %q", got)
	}
	calls, err := rec.Requests(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("two calls were made, read %d: %+v", len(calls), calls)
	}
	if calls[0].Ingest != 2157 || calls[0].Cached != 0 || calls[0].Output != 35 {
		t.Errorf("the first call is the preamble and nothing has been reused yet: %+v", calls[0])
	}
	if calls[1].Cached != 2191 {
		t.Errorf("what the server already held is not read: %+v", calls[1])
	}
	// From the entry before the call to the call: the whole of what that call took.
	if calls[0].Latency != 10*time.Second || calls[1].Latency != 20*time.Second {
		t.Errorf("latencies %v and %v", calls[0].Latency, calls[1].Latency)
	}
	// Peak is the largest context any turn reached, which is what a ceiling is set against.
	if peak, turns := rec.Cost(path); peak != 2240 || turns != 2 {
		t.Errorf("peak %d over %d turns, want 2240 over 2", peak, turns)
	}
}

// A session that never called the model leaves a file with no usage in it, and that is not
// an error: it is the answer that nothing was spent.
func TestPiRecorderReadsASessionThatSpentNothing(t *testing.T) {
	dir := t.TempDir()
	path := piSession(t, dir, `{"type":"session","version":3,"id":"x","timestamp":"2026-08-25T13:08:20Z"}`)
	var rec piRecorder
	if peak, turns := rec.Cost(path); peak != 0 || turns != 0 {
		t.Errorf("peak %d over %d turns, want nothing", peak, turns)
	}
}

// A transcript that cannot be read is not a session that spent nothing. The gate tests the
// peak against a ceiling, so answering 0 there says the session is well inside its budget
// when what happened is that nobody could measure it. chain.Cost answers -1 and this one
// answered 0, against an interface that documented one of them.
func TestPiRecorderAnswersUnmeasuredRatherThanNothing(t *testing.T) {
	var rec piRecorder
	if peak, turns := rec.Cost(filepath.Join(t.TempDir(), "no-such-session.jsonl")); peak != -1 || turns != 0 {
		t.Errorf("peak %d over %d turns, want -1 and 0: nothing to read is not nothing spent",
			peak, turns)
	}
}
