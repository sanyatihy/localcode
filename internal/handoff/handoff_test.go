package handoff

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const doc = `---
id: 0016
---

## Design

A table with a checkbox in it, - [ ] like this, is prose about something else.

## Tasks

- [x] The first box, done
- [ ] The second box, which is the work
- [ ] The third box

## Open questions

- [ ] Not a task box either
`

func TestBoxesAreReadFromTheTasksSectionAlone(t *testing.T) {
	boxes := Boxes([]byte(doc))
	if len(boxes) != 3 {
		t.Fatalf("read %d boxes, want the 3 under ## Tasks: %+v", len(boxes), boxes)
	}
	if !boxes[0].Ticked || boxes[1].Ticked {
		t.Errorf("ticks were not read: %+v", boxes)
	}
	if boxes[1].Text != "The second box, which is the work" {
		t.Errorf("box text is %q", boxes[1].Text)
	}
}

// List order is the order of work, so the topmost unticked box is the whole instruction a
// session gets — and a doc with every box ticked is the only way a driver finishes.
func TestTopmostIsTheFirstUntickedBox(t *testing.T) {
	box, ok := Topmost(Boxes([]byte(doc)))
	if !ok || box.Text != "The second box, which is the work" {
		t.Errorf("topmost is %+v, %v", box, ok)
	}
	if _, ok := Topmost([]Box{{Text: "a", Ticked: true}}); ok {
		t.Error("a doc with every box ticked still offered work")
	}
}

// The driver asks about the box it sent the session to do, not about the topmost one: a
// session that ticked something else has not done what it was asked.
func TestTickedAsksAboutOneNamedBox(t *testing.T) {
	boxes := Boxes([]byte(doc))
	if !Ticked(boxes, "The first box, done") {
		t.Error("a ticked box read as unticked")
	}
	if Ticked(boxes, "The third box") {
		t.Error("an unticked box read as ticked")
	}
	if Ticked(boxes, "a box nobody wrote") {
		t.Error("a box that is not in the doc read as ticked")
	}
}

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

// Peak rather than final, and the whole request rather than its uncached part: what says
// whether the window was the binding constraint is the biggest turn, counted as everything
// that turn put through it.
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

// A transcript line carries whole tool results. Read with a default scanner the first one
// over 64 KB ends the scan without an error, and every turn after it is lost — which reads
// as a small session rather than as a truncated file.
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

// A refused session keeps running and is asked again on the next turn, so this counts
// rather than reporting a flag — and it counts only this session's refusals, because the
// log is one file every session in the checkout appends to.
func TestRefusalsCountThisSessionsOnly(t *testing.T) {
	log := filepath.Join(t.TempDir(), "precompact.jsonl")
	body := ""
	for _, id := range []string{"mine", "other", "mine"} {
		body += fmt.Sprintf("{\"session_id\":%q,\"trigger\":\"auto\"}\n", id)
	}
	if err := os.WriteFile(log, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if n := Refusals(log, "mine"); n != 2 {
		t.Errorf("counted %d refusals, want 2", n)
	}
	// No log means no session here has ever been asked to compact, which is not an error.
	if n := Refusals(filepath.Join(t.TempDir(), "absent.jsonl"), "mine"); n != 0 {
		t.Errorf("a missing log counted %d refusals", n)
	}
}
