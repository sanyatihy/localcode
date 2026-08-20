package prefix

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// This file reads what the server says about itself. The probe measures traffic it wrote,
// and a real session's traffic is nobody's to write: a harness decides what to send and
// reports in units of its own, so the only per-request account of it is the server's log.
//
// Everything below is derived from four lines the server prints for every request. Three
// are read directly. The fourth — how much of the prompt was already there — is not
// printed and is derived, which is why Offset exists.

// How a slot was chosen for a request. The server picks by longest common prefix when one
// slot's contents are close enough to the incoming prompt, and by least-recently-used when
// none is. It is worth recording and worth not over-reading: a request selected by LRU can
// still reuse most of its prompt, because a prefix evicted from the slot is restored from
// the server's host-RAM cache.
const (
	SelectedByLCP = "lcp"
	SelectedByLRU = "lru"
)

// LogRow is one request as the server accounted for it. Separate from Row because it is a
// different kind of evidence: a Row says what a conversation this repo wrote was charged,
// and this says what some session was charged, whoever wrote it.
type LogRow struct {
	Config  string `json:"config"`
	Session string `json:"session"` // what was driving the endpoint

	Index  int `json:"index"`   // position in the log, 1-based
	TaskID int `json:"task_id"` // the server's own id for the request

	SlotSelection string `json:"slot_selection"` // lcp | lru

	PromptTokens    int     `json:"prompt_tokens"`
	IngestedTokens  int     `json:"ingested_tokens"`
	CachedTokens    int     `json:"cached_tokens"`
	HitShare        float64 `json:"hit_share"`
	GeneratedTokens int     `json:"generated_tokens"`

	PromptSeconds float64 `json:"prompt_seconds"`
	GenSeconds    float64 `json:"gen_seconds"`
}

// Offset corrects the one derived quantity. The server reports what it ingested and what
// it generated, but not what it already held; that is the slot's total at release less
// both. The total it prints is one short of the prompt the API accounts for — measured
// against the probe's own rows, where the prompt is known to the token — so a row would
// otherwise under-report reuse by exactly one token per request.
//
// It is a constant with a reason rather than a fudge: a change in the server's accounting
// shows up as this being wrong, and Check is what catches that.
const Offset = 1

// Requests reads a llama-server log and returns one row per request that completed.
//
// A request that the log does not carry to completion is dropped rather than reported with
// zeros: a log is usually read while the server is still running, so the last request is
// routinely half-written, and a truncated one is not a request that cost nothing.
func Requests(r io.Reader, config, session string) ([]LogRow, error) {
	open := map[int]*LogRow{}
	var out []LogRow
	pending := ""

	sc := bufio.NewScanner(r)
	// Server lines are short, but a log may carry a wrapped prompt dump; give the scanner
	// room rather than failing the whole read on one long line.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.Contains(line, "selected slot by LCP similarity"):
			pending = SelectedByLCP
		case strings.Contains(line, "selected slot by LRU"):
			pending = SelectedByLRU
		case strings.Contains(line, "processing task"):
			id, ok := taskID(line)
			if !ok {
				continue
			}
			open[id] = &LogRow{Config: config, Session: session, TaskID: id, SlotSelection: pending}
			pending = ""
		case strings.Contains(line, "prompt eval time"):
			if row := openRow(open, line); row != nil {
				row.IngestedTokens, _ = tokensAfter(line, "/")
				row.PromptSeconds = millis(line) / 1000
			}
		case strings.Contains(line, "eval time"):
			// Reached only when the line is not a *prompt* eval time: the case above
			// matches first, and both lines carry the same suffix.
			if row := openRow(open, line); row != nil {
				row.GeneratedTokens, _ = tokensAfter(line, "/")
				row.GenSeconds = millis(line) / 1000
			}
		case strings.Contains(line, "stop processing"):
			row := openRow(open, line)
			if row == nil {
				continue
			}
			total, ok := valueAfter(line, "n_tokens =")
			if !ok {
				continue
			}
			row.PromptTokens = total + Offset - row.GeneratedTokens
			row.CachedTokens = row.PromptTokens - row.IngestedTokens
			if row.PromptTokens > 0 {
				row.HitShare = float64(row.CachedTokens) / float64(row.PromptTokens)
			}
			row.Index = len(out) + 1
			out = append(out, *row)
			delete(open, row.TaskID)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read log: %w", err)
	}
	return out, nil
}

func openRow(open map[int]*LogRow, line string) *LogRow {
	id, ok := taskID(line)
	if !ok {
		return nil
	}
	return open[id]
}

// taskID reads the "| task N |" the server stamps on every line about a request.
func taskID(line string) (int, bool) {
	v, ok := valueAfter(line, "task ")
	return v, ok
}

// valueAfter reads the first integer following key. Returns false when the key is absent
// or what follows it is not a number, so a line the server prints in a shape this does not
// know is skipped rather than parsed into a zero.
func valueAfter(line, key string) (int, bool) {
	i := strings.Index(line, key)
	if i < 0 {
		return 0, false
	}
	rest := strings.TrimSpace(line[i+len(key):])
	end := 0
	for end < len(rest) && (rest[end] == '-' || (rest[end] >= '0' && rest[end] <= '9')) {
		end++
	}
	if end == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(rest[:end])
	return n, err == nil
}

// tokensAfter reads the token count from a timing line, which reads
// "... time = 52975.28 ms /  5531 tokens (...)".
func tokensAfter(line, sep string) (int, bool) {
	i := strings.Index(line, "ms "+sep)
	if i < 0 {
		return 0, false
	}
	return valueAfter(line[i:], sep)
}

// millis reads the millisecond figure from a timing line.
func millis(line string) float64 {
	i := strings.Index(line, "time =")
	if i < 0 {
		return 0
	}
	f := strings.Fields(strings.TrimSpace(line[i+len("time ="):]))
	if len(f) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0
	}
	return v
}

// Disagreement is one request whose log-derived account differs from what the endpoint
// itself reported. It is what the instrument is checked with, and it names both numbers
// rather than a delta: which one is wrong is the question a disagreement raises.
type Disagreement struct {
	Index                int
	Field                string
	FromLog, FromRequest int
}

func (d Disagreement) String() string {
	return fmt.Sprintf("request %d: log says %s %d, the endpoint reported %d",
		d.Index, d.Field, d.FromLog, d.FromRequest)
}

// Check compares a log's account against rows taken from the endpoint's own replies for
// the same run. The probe knows every prompt to the token, so a run of it is the one place
// this instrument can be held to a known answer — and it is the only reason a derived
// quantity like the cache figure can be trusted on a session nobody scripted.
func Check(fromLog []LogRow, fromRequests []Row) []Disagreement {
	var out []Disagreement
	n := min(len(fromLog), len(fromRequests))
	for i := range n {
		l, r := fromLog[i], fromRequests[i]
		for _, c := range []struct {
			field    string
			log, req int
		}{
			{"prompt", l.PromptTokens, r.PromptTokens},
			{"ingested", l.IngestedTokens, r.IngestedTokens},
			{"cached", l.CachedTokens, r.CachedTokens},
		} {
			if c.log != c.req {
				out = append(out, Disagreement{Index: i + 1, Field: c.field, FromLog: c.log, FromRequest: c.req})
			}
		}
	}
	if len(fromLog) != len(fromRequests) {
		out = append(out, Disagreement{Index: 0, Field: "requests", FromLog: len(fromLog), FromRequest: len(fromRequests)})
	}
	return out
}
