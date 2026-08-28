package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTranscript(t *testing.T, rows ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "0f1e2d3c.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func assistant(in, write, read, out int) string {
	b, _ := json.Marshal(map[string]any{
		"type": "assistant",
		"message": map[string]any{"usage": map[string]int{
			"input_tokens": in, "cache_creation_input_tokens": write,
			"cache_read_input_tokens": read, "output_tokens": out,
		}},
	})
	return string(b)
}

// Peak rather than final, and the whole request rather than its uncached part.
func TestReadSessionTakesTheBiggestTurnNotTheLastOne(t *testing.T) {
	p := writeTranscript(t,
		assistant(2, 22820, 0, 4),
		assistant(1, 0, 40000, 900),
		`{"type":"user","message":{"content":"not a turn"}}`,
		assistant(1, 0, 12000, 10),
	)
	s, err := ReadSession(p)
	if err != nil {
		t.Fatal(err)
	}
	if s.Peak != 40901 {
		t.Errorf("peak is %d, want the second turn's 40901", s.Peak)
	}
	if s.Turns != 3 {
		t.Errorf("counted %d turns, want the 3 assistant rows", s.Turns)
	}
	if s.ID != "0f1e2d3c" {
		t.Errorf("session id is %q", s.ID)
	}
}

// A default scanner ends without an error at the first line over 64 KB, which reads as a
// short session rather than a truncated file.
func TestReadSessionSurvivesATranscriptLineLargerThanAScannerBuffer(t *testing.T) {
	huge, _ := json.Marshal(map[string]any{
		"type":    "user",
		"message": map[string]any{"content": strings.Repeat("x", 200_000)},
	})
	s, err := ReadSession(writeTranscript(t, assistant(1, 0, 0, 1), string(huge), assistant(1, 0, 99, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if s.Turns != 2 || s.Peak != 101 {
		t.Errorf("read %d turns and a peak of %d after a 200 KB line", s.Turns, s.Peak)
	}
}

// call is one response as Claude Code files it: a row per content block, every one of them
// carrying the whole call's usage and the same message id.
func call(id, at string, in, write, read, out int) string {
	b, _ := json.Marshal(map[string]any{
		"type": "assistant", "timestamp": at,
		"message": map[string]any{"id": id, "usage": map[string]int{
			"input_tokens": in, "cache_creation_input_tokens": write,
			"cache_read_input_tokens": read, "output_tokens": out,
		}},
	})
	return string(b)
}

func at(ts string) string {
	return `{"type":"user","timestamp":"` + ts + `","message":{"content":"a tool result"}}`
}

// A turn answering with text and two tool calls files three rows carrying one usage.
// Counted per row it charges that call three times, which is how a chain's generated
// tokens stop matching the server's own counter.
func TestRequestsAreCountedPerCallAndNotPerContentBlock(t *testing.T) {
	calls, err := Requests(writeTranscript(t,
		at("2026-08-23T06:41:06.080Z"),
		call("msg_1", "2026-08-23T06:41:59.926Z", 4165, 0, 0, 86),
		at("2026-08-23T06:42:00.263Z"),
		call("msg_2", "2026-08-23T06:43:15.874Z", 1342, 0, 6442, 176),
		call("msg_2", "2026-08-23T06:43:15.879Z", 1342, 0, 6442, 176),
		call("msg_2", "2026-08-23T06:43:15.880Z", 1342, 0, 6442, 176),
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("read %d calls from 4 assistant rows, want 2: %+v", len(calls), calls)
	}
	if calls[1].Output != 176 || calls[1].Ingest != 1342 || calls[1].Cached != 6442 {
		t.Errorf("the second call is %+v", calls[1])
	}
}

// The prompt the server had to read is what was not already held, which is the uncached
// input plus whatever the call wrote into the cache.
func TestRequestsChargeIngestForWhatWasNotCached(t *testing.T) {
	calls, err := Requests(writeTranscript(t,
		at("2026-08-23T06:41:06.000Z"),
		call("msg_1", "2026-08-23T06:41:07.000Z", 300, 1000, 6442, 20),
	))
	if err != nil {
		t.Fatal(err)
	}
	if calls[0].Ingest != 1300 || calls[0].Cached != 6442 {
		t.Errorf("ingest %d and cached %d, want 1300 and 6442", calls[0].Ingest, calls[0].Cached)
	}
}

// A call's clock starts when the harness finished the row before it, since that is the last
// thing that happened before it was sent.
func TestRequestsMeasureACallFromTheRowBeforeIt(t *testing.T) {
	calls, err := Requests(writeTranscript(t,
		at("2026-08-23T06:41:06.000Z"),
		call("msg_1", "2026-08-23T06:41:59.500Z", 4165, 0, 0, 86),
		at("2026-08-23T06:42:00.000Z"),
		call("msg_2", "2026-08-23T06:42:20.000Z", 1001, 0, 4252, 70),
	))
	if err != nil {
		t.Fatal(err)
	}
	if calls[0].Latency != 53500*time.Millisecond {
		t.Errorf("the first call took %v, want 53.5s", calls[0].Latency)
	}
	if calls[1].Latency != 20*time.Second {
		t.Errorf("the second call took %v, want the 20s since the tool result", calls[1].Latency)
	}
}
