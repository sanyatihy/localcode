package prefix

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/sanyatihy/localcode/internal/build"
)

// This file reads what the server says about itself, because a real session's traffic is
// nobody's to script. Everything below comes from four lines the server prints per
// request: three are read directly, and the fourth — how much of the prompt was already
// there — is derived, which is why Offset exists.

// How a slot was chosen: longest common prefix when one slot's contents are close enough,
// least-recently-used when none is. Worth not over-reading — an LRU selection can still
// reuse most of its prompt, because an evicted prefix is restored from host RAM.
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

	// What host-RAM prompt cache served this request, read from scripts/serve.sh's banner
	// at the head of the log. "default" means no config named one, which is what every
	// measurement in this repo before 2026-08-25 was taken under. Empty means the log did
	// not start with a banner, so the server was not started by that script.
	CacheRAM string `json:"cache_ram"`

	PromptTokens    int     `json:"prompt_tokens"`
	IngestedTokens  int     `json:"ingested_tokens"`
	CachedTokens    int     `json:"cached_tokens"`
	HitShare        float64 `json:"hit_share"`
	GeneratedTokens int     `json:"generated_tokens"`

	PromptSeconds float64 `json:"prompt_seconds"`
	GenSeconds    float64 `json:"gen_seconds"`

	// Driver is the build of this repository that wrote the row: a short revision, with
	// `+modified` when the tree it was built from was not clean, and `unknown` when the
	// build recorded none. A row is evidence only if somebody can get back to the code that
	// produced it, and the driver's own arithmetic has moved under rows before.
	Driver string `json:"driver"`
}

// Offset corrects the one derived quantity. The slot's total at release runs one token
// short of the prompt the API accounts for, measured against the probe's rows where every
// prompt is known to the token. A change in the server's accounting shows up as this being
// wrong, and Check is what catches it.
const Offset = 1

// possible reports whether a row's account could have come from a request the server
// served. The prompt figure is derived rather than printed — `n_tokens` at release, plus
// Offset, less what was generated — so a log whose lines do not pair produces arithmetic
// instead of a measurement: `n_tokens = 0` against 7 generated yields a prompt of -6, and a
// cache figure of -6 behind it.
func (r LogRow) possible() bool {
	return r.PromptTokens > 0 && r.IngestedTokens >= 0 &&
		r.CachedTokens >= 0 && r.GeneratedTokens >= 0
}

// Requests reads a llama-server log and returns one row per request that completed, and how
// many it read that could not have happened.
//
// A request that the log does not carry to completion is dropped rather than reported with
// zeros: a log is usually read while the server is still running, so the last request is
// routinely half-written, and a truncated one is not a request that cost nothing. That drop
// is expected and is not counted — the count is for rows the arithmetic refuses, which is a
// fact about the log rather than about the reading, and reporting fewer requests than a log
// holds without saying so is how an instrument lies quietly.
func Requests(r io.Reader, config, session string) (rows []LogRow, impossible int, err error) {
	open := map[int]*LogRow{}
	var out []LogRow
	pending := ""
	cacheRAM := ""

	sc := bufio.NewScanner(r)
	// Server lines are short, but a log may carry a wrapped prompt dump; give the scanner
	// room rather than failing the whole read on one long line.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "serving "):
			cacheRAM = fieldAfter(line, "cache-ram=")
		case strings.Contains(line, "selected slot by LCP similarity"):
			pending = SelectedByLCP
		case strings.Contains(line, "selected slot by LRU"):
			pending = SelectedByLRU
		case strings.Contains(line, "processing task"):
			id, ok := taskID(line)
			if !ok {
				continue
			}
			open[id] = &LogRow{Config: config, Session: session, TaskID: id,
				SlotSelection: pending, CacheRAM: cacheRAM, Driver: build.Revision()}
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
			delete(open, row.TaskID)
			// Dropped rather than clamped: a zero here would put a row in the file saying
			// a request ingested nothing, which is the finding this instrument exists to
			// report. The reading has to be absent rather than wrong.
			if !row.possible() {
				impossible++
				continue
			}
			row.HitShare = float64(row.CachedTokens) / float64(row.PromptTokens)
			row.Index = len(out) + 1
			out = append(out, *row)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, 0, fmt.Errorf("read log: %w", err)
	}
	return out, impossible, nil
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

// fieldAfter reads the whitespace-delimited word following key, and "" when key is absent.
func fieldAfter(line, key string) string {
	i := strings.Index(line, key)
	if i < 0 {
		return ""
	}
	f := strings.Fields(line[i+len(key):])
	if len(f) == 0 {
		return ""
	}
	return f[0]
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
