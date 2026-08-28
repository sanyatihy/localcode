// Package handoff reads what a finished session cost, out of the transcript it wrote.
package handoff

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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

// Request is one call a session made to the model, as its own transcript records it.
type Request struct {
	Ingest  int           // prompt tokens the server had to read
	Cached  int           // prompt tokens it already held
	Output  int           // tokens generated
	Latency time.Duration // the row before it to this one: the whole call, prefill included
}

// Requests reads a transcript's model calls, in the order they were made.
//
// One call, not one row: Claude Code files an assistant row per content block and all of
// them carry that call's usage, so counting rows charges a turn that answered with text
// and two tool calls three times. The message id is what makes them one call again —
// summed that way, ingest and output match the server's own counters on both chains of
// 0025 exactly.
//
// Latency is measured from the row before the call, which is what the harness had
// finished when it sent it. A transcript records no first-token time, so one call's
// prefill, decode and whatever the harness spent between them arrive as one number.
func Requests(transcript string) ([]Request, error) {
	f, err := os.Open(transcript)
	if err != nil {
		return nil, fmt.Errorf("transcript: %w", err)
	}
	defer func() { _ = f.Close() }()

	var calls []Request
	seen := map[string]bool{}
	var prev time.Time
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scan.Scan() {
		var row struct {
			Type      string `json:"type"`
			Timestamp string `json:"timestamp"`
			Message   struct {
				ID    string `json:"id"`
				Usage usage  `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(scan.Bytes(), &row); err != nil {
			continue // bookkeeping rows this does not model
		}
		at, err := time.Parse(time.RFC3339, row.Timestamp)
		if err != nil {
			continue // a row with no clock cannot bound a call
		}
		if row.Type != "assistant" {
			if at.After(prev) {
				prev = at
			}
			continue
		}
		if row.Message.ID == "" || seen[row.Message.ID] {
			continue
		}
		seen[row.Message.ID] = true
		u := row.Message.Usage
		call := Request{Ingest: u.Input + u.CacheWrite, Cached: u.CacheRead, Output: u.Output}
		if !prev.IsZero() {
			call.Latency = at.Sub(prev)
		}
		calls = append(calls, call)
		prev = at
	}
	if err := scan.Err(); err != nil {
		return calls, fmt.Errorf("read %s: %w", transcript, err)
	}
	return calls, nil
}
