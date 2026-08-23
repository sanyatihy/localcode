package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sanyatihy/localcode/internal/handoff"
)

// chainOnDisk writes a chain the way a run leaves one: a session per directory with the
// counter the transcript is found through, a transcript under the config directory, and
// the supervisor's own row per session in `sessions.jsonl`.
func chainOnDisk(t *testing.T, id string, sessions ...[]string) string {
	t.Helper()
	repo := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	config := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", config)
	t.Chdir(repo)

	state, err := repoState(repo)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(state, "chains", id)
	var lines strings.Builder
	for n, rows := range sessions {
		sessionID := fmt.Sprintf("0000000%d-dead-beef-cafe-000000000000", n+1)
		sessionDir := filepath.Join(dir, fmt.Sprintf("%02d", n+1))
		if err := os.MkdirAll(sessionDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sessionDir, "calls-"+sessionID), []byte("."), 0o644); err != nil {
			t.Fatal(err)
		}
		project := filepath.Join(config, "projects", "a-repo")
		if err := os.MkdirAll(project, 0o755); err != nil {
			t.Fatal(err)
		}
		body := strings.Join(rows, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(project, sessionID+".jsonl"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		line, _ := json.Marshal(row{Chain: id, Session: n + 1, Seconds: 100})
		lines.Write(line)
		lines.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "sessions.jsonl"), []byte(lines.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func transcriptCall(id, at string, in, read, out int) string {
	b, _ := json.Marshal(map[string]any{
		"type": "assistant", "timestamp": at,
		"message": map[string]any{"id": id, "usage": map[string]int{
			"input_tokens": in, "cache_creation_input_tokens": 0,
			"cache_read_input_tokens": read, "output_tokens": out,
		}},
	})
	return string(b)
}

func transcriptRow(at string) string {
	return `{"type":"user","timestamp":"` + at + `","message":{"content":"a tool result"}}`
}

// The four numbers a decision about a context or a mechanism is made on, and the one thing
// the preamble is: the first call's prompt, paid again by every session in the chain.
func TestAccountReportsWhatAWholeChainCost(t *testing.T) {
	chainOnDisk(t, "20260823-104105",
		[]string{
			transcriptRow("2026-08-23T06:41:00.000Z"),
			transcriptCall("a1", "2026-08-23T06:41:40.000Z", 4000, 0, 80),
			transcriptRow("2026-08-23T06:41:41.000Z"),
			transcriptCall("a2", "2026-08-23T06:42:01.000Z", 500, 4080, 120),
		},
		[]string{
			transcriptRow("2026-08-23T06:50:00.000Z"),
			transcriptCall("b1", "2026-08-23T06:50:45.000Z", 4400, 0, 90),
		},
	)
	var out strings.Builder
	if code, err := accountHere(&out, ""); code != 0 || err != nil {
		t.Fatalf("account = %d, %v", code, err)
	}
	for _, want := range []string{
		"chain 20260823-104105 — 2 sessions, 200 s",
		"generated  290 tokens",
		"ingested   8900 tokens, 8400 of it preamble",
		"reused     4080 tokens",
		"105.0 s inside a call to the model, 95.0 s outside one",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the report does not say %q:\n%s", want, out.String())
		}
	}
}

// The newest chain by default, because that is the one an operator has just watched run.
func TestAccountTakesTheNewestChainWhenNoneIsNamed(t *testing.T) {
	dir := chainOnDisk(t, "20260823-104105", []string{transcriptRow("2026-08-23T06:41:00.000Z")})
	if err := os.MkdirAll(filepath.Join(filepath.Dir(dir), "20260101-000000"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if code, err := accountHere(&out, ""); code != 0 || err != nil {
		t.Fatalf("account = %d, %v", code, err)
	}
	if !strings.Contains(out.String(), "chain 20260823-104105") {
		t.Errorf("accounted the wrong chain:\n%s", out.String())
	}
}

func TestAccountRefusesAChainThatIsNotHere(t *testing.T) {
	chainOnDisk(t, "20260823-104105", []string{transcriptRow("2026-08-23T06:41:00.000Z")})
	code, err := accountHere(io.Discard, "20260101-000000")
	if code != 2 || err == nil {
		t.Fatalf("account = %d, %v; want a refusal", code, err)
	}
	if !strings.Contains(err.Error(), "localcode sessions") {
		t.Errorf("the refusal does not name the command that lists them: %v", err)
	}
}

// Two rates over many calls, each call mixing them differently: what is recovered is what
// the calls were built from.
func TestFitRatesRecoversThePairTheCallsWereBuiltFrom(t *testing.T) {
	const prefillRate, decodeRate = 100.0, 8.0
	var calls []handoff.Request
	for _, c := range [][2]int{{4000, 80}, {500, 200}, {300, 40}, {1200, 150}} {
		seconds := float64(c[0])/prefillRate + float64(c[1])/decodeRate
		calls = append(calls, handoff.Request{
			Ingest: c[0], Output: c[1],
			Latency: time.Duration(seconds * float64(time.Second)),
		})
	}
	prefill, decode, ok := fitRates(calls)
	if !ok {
		t.Fatal("four calls that mix the two rates did not separate them")
	}
	if diff := prefill - prefillRate; diff > 0.5 || diff < -0.5 {
		t.Errorf("prefill fitted at %.2f tok/s, want %.0f", prefill, prefillRate)
	}
	if diff := decode - decodeRate; diff > 0.05 || diff < -0.05 {
		t.Errorf("decode fitted at %.2f tok/s, want %.0f", decode, decodeRate)
	}
}

// Calls whose prompts and replies grew together fit any pair of rates that sums right, so
// the honest answer is that these calls do not decide it.
func TestFitRatesRefusesCallsThatCannotSeparateThem(t *testing.T) {
	var calls []handoff.Request
	for _, n := range []int{1, 2, 3, 4} {
		calls = append(calls, handoff.Request{
			Ingest: 100 * n, Output: 10 * n, Latency: time.Duration(n) * 2 * time.Second,
		})
	}
	if prefill, decode, ok := fitRates(calls); ok {
		t.Errorf("fitted %.2f and %.2f tok/s from calls that cannot decide them", prefill, decode)
	}
	if _, _, ok := fitRates(calls[:1]); ok {
		t.Error("one call fitted two rates")
	}
}

// A chain's totals hide the session that spent them badly, so the columns repeat per
// session — and a session whose own calls cannot decide a rate says so rather than
// borrowing the chain's, which was fitted over other sessions' calls too.
func TestAccountRepeatsTheColumnsPerSession(t *testing.T) {
	chainOnDisk(t, "20260823-104105",
		[]string{
			transcriptRow("2026-08-23T06:41:00.000Z"),
			transcriptCall("a1", "2026-08-23T06:41:40.000Z", 4000, 0, 80),
			transcriptRow("2026-08-23T06:41:41.000Z"),
			transcriptCall("a2", "2026-08-23T06:42:01.000Z", 500, 4080, 120),
		},
		[]string{
			transcriptRow("2026-08-23T06:50:00.000Z"),
			transcriptCall("b1", "2026-08-23T06:50:45.000Z", 4400, 0, 90),
		},
	)
	var out strings.Builder
	if code, err := accountHere(&out, ""); code != 0 || err != nil {
		t.Fatalf("account = %d, %v", code, err)
	}
	rows := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	first, second := rows[len(rows)-2], rows[len(rows)-1]
	for _, want := range []string{"2", "100", "60.0", "4000", "4500", "4080", "200", "7.33"} {
		if !strings.Contains(first, want) {
			t.Errorf("session 1 does not report %q: %q", want, first)
		}
	}
	if !strings.HasSuffix(second, "—") {
		t.Errorf("a session of one call reported a decode rate: %q", second)
	}
}

// One session is the whole chain, so repeating it under itself says nothing twice.
func TestAccountLeavesOutTheBreakdownOfASingleSession(t *testing.T) {
	chainOnDisk(t, "20260823-104105", []string{
		transcriptRow("2026-08-23T06:41:00.000Z"),
		transcriptCall("a1", "2026-08-23T06:41:40.000Z", 4000, 0, 80),
	})
	var out strings.Builder
	if code, err := accountHere(&out, ""); code != 0 || err != nil {
		t.Fatalf("account = %d, %v", code, err)
	}
	if strings.Contains(out.String(), "session  calls") {
		t.Errorf("a chain of one session printed a per-session table:\n%s", out.String())
	}
}

// The point of reading a chain's own files: the session doing the work is the one with no
// row in `sessions.jsonl`, because that row is appended when the session ends.
func TestAccountReadsAChainThatIsStillRunning(t *testing.T) {
	dir := chainOnDisk(t, "20260823-104105",
		[]string{
			transcriptRow("2026-08-23T06:41:00.000Z"),
			transcriptCall("a1", "2026-08-23T06:41:40.000Z", 4000, 0, 80),
		},
		[]string{
			transcriptRow("2026-08-23T06:50:00.000Z"),
			transcriptCall("b1", "2026-08-23T06:50:45.000Z", 4400, 0, 90),
			transcriptRow("2026-08-23T06:50:46.000Z"),
			transcriptCall("b2", "2026-08-23T06:51:16.000Z", 600, 4490, 200),
		},
	)
	endFirstSessionOnly(t, dir)

	var out strings.Builder
	if code, err := accountHere(&out, ""); code != 0 || err != nil {
		t.Fatalf("account = %d, %v", code, err)
	}
	for _, want := range []string{
		"2 sessions, 1 still running, 100 s recorded",
		"generated  370 tokens",
		"ingested   9000 tokens, 8400 of it preamble",
		"115.0 s inside a call to the model, and 1 session still to be timed",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the report does not say %q:\n%s", want, out.String())
		}
	}
	last := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if !strings.Contains(last[len(last)-1], "—") {
		t.Errorf("the running session was given a wall clock: %q", last[len(last)-1])
	}
}

// A chain whose first session has not ended has written no `sessions.jsonl` at all, which
// is the answer that none of them has ended rather than a file that could not be read.
func TestAccountReadsAChainBeforeItsFirstSessionEnds(t *testing.T) {
	dir := chainOnDisk(t, "20260823-104105", []string{
		transcriptRow("2026-08-23T06:41:00.000Z"),
		transcriptCall("a1", "2026-08-23T06:41:40.000Z", 4000, 0, 80),
	})
	if err := os.Remove(filepath.Join(dir, "sessions.jsonl")); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if code, err := accountHere(&out, ""); code != 0 || err != nil {
		t.Fatalf("account = %d, %v", code, err)
	}
	if !strings.Contains(out.String(), "1 session, 1 still running, 0 s recorded") {
		t.Errorf("a chain with no sessions.jsonl was not reported as running:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "generated  80 tokens") {
		t.Errorf("the running session's calls were not counted:\n%s", out.String())
	}
}

// A transcript is being appended to while this reads it, so its last line is as likely to
// be half a row as a whole one.
func TestAccountSurvivesATranscriptBeingWritten(t *testing.T) {
	dir := chainOnDisk(t, "20260823-104105", []string{
		transcriptRow("2026-08-23T06:41:00.000Z"),
		transcriptCall("a1", "2026-08-23T06:41:40.000Z", 4000, 0, 80),
		transcriptRow("2026-08-23T06:41:41.000Z"),
		`{"type":"assistant","timestamp":"2026-08-23T06:42:0`,
	})
	if err := os.Remove(filepath.Join(dir, "sessions.jsonl")); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if code, err := accountHere(&out, ""); code != 0 || err != nil {
		t.Fatalf("account = %d, %v", code, err)
	}
	if !strings.Contains(out.String(), "generated  80 tokens") {
		t.Errorf("a half-written row cost the rows before it:\n%s", out.String())
	}
}

// endFirstSessionOnly leaves the chain's record saying one session has ended, which is what
// it says while the second one works.
func endFirstSessionOnly(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "sessions.jsonl")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	first := strings.SplitN(string(body), "\n", 2)[0] + "\n"
	if err := os.WriteFile(path, []byte(first), 0o644); err != nil {
		t.Fatal(err)
	}
}
