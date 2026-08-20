// Package handoff reads the two things a driver needs to run one task box across more than
// one session: which box a feature doc is on, and what a finished session cost.
//
// Nothing here summarises anything. What survives a feature is already in the repo, what a
// session was in the middle of is in HANDOFF.md, and what is left — how big the session got
// and how long it ran — is in the transcript it wrote.
package handoff

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Box is one entry of a feature doc's `## Tasks` list. List order is the order of work, so
// position is meaning and the slice keeps it.
type Box struct {
	Text   string
	Ticked bool
}

// Boxes returns the doc's task boxes in order.
//
// Only the `## Tasks` section is read. A checkbox anywhere else in a doc is prose about
// something else, and counting it would leave a driver working a box nobody wrote.
func Boxes(doc []byte) []Box {
	var boxes []Box
	inTasks := false
	for _, line := range strings.Split(string(doc), "\n") {
		if strings.HasPrefix(line, "## ") {
			inTasks = strings.TrimSpace(line) == "## Tasks"
			continue
		}
		if !inTasks {
			continue
		}
		switch {
		case strings.HasPrefix(line, "- [ ] "):
			boxes = append(boxes, Box{Text: strings.TrimSpace(line[6:])})
		case strings.HasPrefix(line, "- [x] "):
			boxes = append(boxes, Box{Text: strings.TrimSpace(line[6:]), Ticked: true})
		}
	}
	return boxes
}

// Topmost returns the box a session should be working: the first unticked one. The second
// return is false when every box is ticked, which is the only way a driver finishes.
func Topmost(boxes []Box) (Box, bool) {
	for _, b := range boxes {
		if !b.Ticked {
			return b, true
		}
	}
	return Box{}, false
}

// Ticked reports whether the named box is ticked now. The driver asks about the box it
// started on rather than about the topmost one, because a session that ticked something
// else has not done what it was sent to do.
func Ticked(boxes []Box, text string) bool {
	for _, b := range boxes {
		if b.Text == text {
			return b.Ticked
		}
	}
	return false
}

// Session is what a transcript records about the session that wrote it.
type Session struct {
	ID    string
	Turns int
	Peak  int
}

// usage is the subset of a transcript's token counts that says how big a request was. All
// four are the same context: what was sent, whatever the cache did with it, plus what came
// back — a turn's cost against the window is the sum, not the uncached part of it.
type usage struct {
	Input      int `json:"input_tokens"`
	CacheWrite int `json:"cache_creation_input_tokens"`
	CacheRead  int `json:"cache_read_input_tokens"`
	Output     int `json:"output_tokens"`
}

// ReadSession returns the session a transcript belongs to, its turn count, and the largest
// context any one of its turns occupied.
//
// Peak rather than final: a session that was refused a compaction keeps growing, so what
// says whether the window was the binding constraint is the biggest turn, and the last turn
// is only the biggest when nothing went wrong.
func ReadSession(transcript string) (Session, error) {
	f, err := os.Open(transcript)
	if err != nil {
		return Session{}, fmt.Errorf("transcript: %w", err)
	}
	defer func() { _ = f.Close() }()

	s := Session{ID: strings.TrimSuffix(filepath.Base(transcript), ".jsonl")}
	scan := bufio.NewScanner(f)
	// A transcript line carries whole tool results, which are routinely larger than the
	// scanner's default 64 KB and would end the scan silently at the first one.
	scan.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scan.Scan() {
		var row struct {
			Type    string `json:"type"`
			Message struct {
				Usage usage `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(scan.Bytes(), &row); err != nil {
			// A transcript holds bookkeeping rows this does not model. One that will not
			// parse is not a reason to lose the rest of the session.
			continue
		}
		if row.Type != "assistant" {
			continue
		}
		s.Turns++
		u := row.Message.Usage
		if total := u.Input + u.CacheWrite + u.CacheRead + u.Output; total > s.Peak {
			s.Peak = total
		}
	}
	if err := scan.Err(); err != nil {
		return s, fmt.Errorf("read %s: %w", transcript, err)
	}
	return s, nil
}

// Refusals counts the compactions refused during one session. The PreCompact hook appends
// a row per refusal and a refused session keeps running, so this is a count and not a flag.
//
// A log that is not there is not an error: it means no session in this checkout has ever
// been asked to compact.
func Refusals(log, session string) int {
	f, err := os.Open(log)
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()

	n := 0
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		var row struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal(scan.Bytes(), &row); err == nil && row.SessionID == session {
			n++
		}
	}
	return n
}
