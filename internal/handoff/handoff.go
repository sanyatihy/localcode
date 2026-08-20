// Package handoff reads what a driver needs to run one task box across several sessions:
// which box a doc is on, and what a finished session cost.
package handoff

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Box is one entry of a feature doc's `## Tasks` list, in list order.
type Box struct {
	Text   string
	Ticked bool
}

// Boxes reads the `## Tasks` section only: a checkbox elsewhere is prose about something
// else.
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

// Topmost returns the first unticked box, or false when every box is ticked.
func Topmost(boxes []Box) (Box, bool) {
	for _, b := range boxes {
		if !b.Ticked {
			return b, true
		}
	}
	return Box{}, false
}

// Ticked reports whether one named box is ticked. The driver asks about the box it sent
// the session to do, not the topmost one.
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

// usage is a turn's cost against the window. All four count: the cache changes what is
// paid for, not what is in the context.
type usage struct {
	Input      int `json:"input_tokens"`
	CacheWrite int `json:"cache_creation_input_tokens"`
	CacheRead  int `json:"cache_read_input_tokens"`
	Output     int `json:"output_tokens"`
}

// ReadSession returns a transcript's session, turn count, and largest turn.
//
// Peak rather than final: the last turn is the biggest only when nothing went wrong.
func ReadSession(transcript string) (Session, error) {
	f, err := os.Open(transcript)
	if err != nil {
		return Session{}, fmt.Errorf("transcript: %w", err)
	}
	defer func() { _ = f.Close() }()

	s := Session{ID: strings.TrimSuffix(filepath.Base(transcript), ".jsonl")}
	scan := bufio.NewScanner(f)
	// A transcript line holds a whole tool result. The default 64 KB would end the scan
	// silently at the first big one.
	scan.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scan.Scan() {
		var row struct {
			Type    string `json:"type"`
			Message struct {
				Usage usage `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(scan.Bytes(), &row); err != nil {
			continue // bookkeeping rows this does not model
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

// Refusals counts one session's refused compactions. A count, not a flag: a refused
// session keeps running and is asked again next turn. No log means none were ever asked.
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
