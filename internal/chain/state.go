package chain

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/sanyatihy/localcode/internal/handoff"
)

// Counter reads one of a session's counters, which is the size of a file the hooks append
// a byte to. Named files rather than dotfiles: a counter `ls` does not show is one a
// reader concludes never fired, which happened twice while this was being measured.
func Counter(dir, name string) int {
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		return 0
	}
	return int(info.Size())
}

// Bump raises a counter and returns what it now reads, which is this caller's own place in
// the queue.
//
// A byte appended to a file opened O_APPEND is atomic, and the whole counter is that file's
// size. Read-modify-write loses counts here: the harness runs one turn's tool calls
// concurrently, so several hooks are deciding at once — measured, eleven permitted calls
// left a counter reading eight. Bumping before deciding is what makes each of those hooks
// see a different number.
func Bump(dir, name string) int {
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return Counter(dir, name)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write([]byte{'.'}); err != nil {
		return Counter(dir, name)
	}
	// The offset after the write, not the file's size afterwards: between the two another
	// hook appends, and two callers reading the size get the same number back.
	at, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return Counter(dir, name)
	}
	return int(at)
}

// CallsFile and HandoffsFile name the two counters one session keeps. Per session id, so
// a directory reused by a chain never carries one session's spending into the next.
func CallsFile(session string) string    { return "calls-" + safe(session) }
func HandoffsFile(session string) string { return "handoffs-" + safe(session) }
func StopFile(session string) string     { return "stops-" + safe(session) }

// BatchName names the one file a session's turn counter lives in. One file rather than one
// per turn: each call writes the reading it counted against, so a turn that reports another
// is a line the file did not carry before. Keyed by the reading, the name was a
// cache-buster and the store grew with every turn a chain ever took.
func BatchName(session string) string { return "batch-" + safe(session) }

// BumpBatch counts a call against the turn that reported this peak, and returns this
// caller's place in that turn the way Bump returns its place in a session's queue.
//
// The record is appended and nothing is rewritten. A header would be, and the rewrite is
// contended exactly where it must not be: the first call of a turn is the one the harness
// issues alongside the rest, so every hook finds no header and every one writes it —
// measured, 32 concurrent calls of one turn were counted as one.
func BumpBatch(dir, session string, r Reading) int {
	path := filepath.Join(dir, BatchName(session))
	rec := batchRecord(r)
	at := int64(0)
	if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		defer func() { _ = f.Close() }()
		if _, err := f.Write([]byte(rec)); err == nil {
			// The offset after the write, for Bump's reason: between the write and a read
			// of the size another hook appends, and two callers read the same number back.
			if off, err := f.Seek(0, io.SeekCurrent); err == nil {
				at = off
			}
		}
	}
	return batchSpent(path, rec, at)
}

// Reading is what a turn's calls are counted against: the context the gate measured, and
// which turn of the transcript reported it. The turn is what tells two turns at one context
// apart — without it the second shares the first's counter and is refused every call it
// makes. It is 0 for a harness that reports its own context, which has no transcript here
// to count turns in.
type Reading struct {
	Peak int
	Turn int
}

// batchRecord is what one call writes: the reading it was decided against, on a line of
// its own.
func batchRecord(r Reading) string { return fmt.Sprintf("%d %d\n", r.Peak, r.Turn) }

// batchSpent counts the calls this turn has taken, up to the caller's own record and no
// further. A record past it belongs to a hook deciding at the same moment, and the file
// before it cannot change — so each of a turn's concurrent callers reads a different
// number and the count is nobody's read-modify-write.
//
// A turn is the run of records that end the prefix and match, because the reading is what
// does not move while a turn's calls run. `at` is 0 when the write did not land, and then
// the whole file is what the caller knows.
func batchSpent(path, rec string, at int64) int {
	body, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	if at > 0 && at < int64(len(body)) {
		body = body[:at]
	}
	n := 0
	for len(body) >= len(rec) && string(body[len(body)-len(rec):]) == rec {
		rest := body[:len(body)-len(rec)]
		// A record ends the line it is on, so anything else before it is a longer reading
		// this one is the tail of: `15100 2` must not be read as a call at `5100 2`.
		if len(rest) > 0 && rest[len(rest)-1] != '\n' {
			break
		}
		body, n = rest, n+1
	}
	return n
}

// safe keeps a session id from naming a file outside the state directory. The id comes
// from the harness rather than from a person, which is a reason to check it once here
// rather than to trust it in three places.
func safe(session string) string {
	if session == "" {
		return "unnamed"
	}
	return filepath.Base(filepath.Clean(session))
}

// Cost is what a transcript says its session spent: the largest context any turn reached,
// and how many turns there were. Peak is -1 when there is no transcript to read, which is
// the case for the first call of a session, before the file exists.
//
// It reads through internal/handoff so a session's cost has one definition: what a
// supervisor reports and what the gate enforces cannot drift apart if they are one count.
func Cost(transcript string) (peak, turns int) {
	if transcript == "" {
		return -1, 0
	}
	s, err := handoff.ReadSession(transcript)
	if err != nil {
		return -1, 0
	}
	return s.Peak, s.Turns
}

// Peak is Cost's first answer, which is the only one the gate needs.
func Peak(transcript string) int {
	peak, _ := Cost(transcript)
	return peak
}

// SessionIDIn reads back the session id the gate counted against, from the counter it
// named after it. A directory can outlive the session that filled it, and a count carried
// into the next one would spend a budget nobody used — so the id is what the file says
// rather than what the directory is called. A session that called no tool leaves no
// counter, and this answers "".
func SessionIDIn(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if id, ok := strings.CutPrefix(e.Name(), "calls-"); ok {
			return id
		}
	}
	return ""
}

// Read returns a handoff's contents, or nil when there is none.
func Read(path string) []byte {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return body
}

// Chains lists a repository's chains, newest first. The id is a timestamp, so sorting the
// names is sorting the runs — which is what `-continue` needs and what `localcode
// sessions` prints.
//
// The disambiguating suffix is compared as a number, not as text. Two chains started in
// one second are `…-150405` and `…-150405-2`, and as strings `-10` sorts before `-2`: past
// nine of them `-continue` would take the wrong one.
func Chains(state string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(state, "chains"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	slices.SortFunc(ids, func(a, b string) int { return -compareChainID(a, b) })
	return ids, nil
}

// compareChainID orders two ids oldest first: by the timestamp they open with, and then by
// the suffix as a number. An id this does not recognise falls back to the whole string, so
// a directory nobody here named still sorts somewhere stable.
func compareChainID(a, b string) int {
	baseA, nA := chainIDParts(a)
	baseB, nB := chainIDParts(b)
	if c := strings.Compare(baseA, baseB); c != 0 {
		return c
	}
	return cmp.Compare(nA, nB)
}

// chainIDParts splits `20260827-150405-2` into its timestamp and its 2. A bare id is 1,
// which is what `newChain` gives the first chain of a second.
func chainIDParts(id string) (base string, n int) {
	base, suffix, ok := strings.Cut(id, "-")
	rest, tail, hasTail := strings.Cut(suffix, "-")
	if !ok {
		return id, 1
	}
	if !hasTail {
		return id, 1
	}
	n, err := strconv.Atoi(tail)
	if err != nil {
		return id, 1
	}
	return base + "-" + rest, n
}

// Sessions lists the sessions a chain holds, in the order they ran. A numbered directory
// is the record that a session started, and it is made before the session runs — so this
// sees the one that is still running, which `sessions.jsonl` cannot: that file's row is
// appended once the session has ended.
func Sessions(dir string) []int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var ns []int
	for _, e := range entries {
		if n, err := strconv.Atoi(e.Name()); err == nil && e.IsDir() {
			ns = append(ns, n)
		}
	}
	sort.Ints(ns)
	return ns
}

// NextSession is the number the next session in a chain takes. Sessions are numbered
// directories, so the chain's own layout is what answers this rather than a counter that
// could disagree with it.
func NextSession(dir string) int {
	ns := Sessions(dir)
	if len(ns) == 0 {
		return 1
	}
	return ns[len(ns)-1] + 1
}

// LatestHandoff is the newest handoff a chain holds, or "" when it holds none. The newest
// rather than the one in the last directory: a session that wrote none leaves an empty
// directory behind, and what the next one inherits is the last thing anybody wrote.
func LatestHandoff(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	best, at := "", -1
	for _, e := range entries {
		n, err := strconv.Atoi(e.Name())
		if err != nil || !e.IsDir() || n <= at {
			continue
		}
		path := filepath.Join(dir, e.Name(), HandoffName)
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			best, at = path, n
		}
	}
	return best
}

// EndingName is what a chain's ending is written to, beside the rows of its sessions.
const EndingName = "chain.json"

// Reason is how a chain stopped, in one of five fixed words. A word rather than prose,
// because the point is that a program can test it, and prose is what the handoff is for.
type Reason string

const (
	Finished    Reason = "finished"
	Stalled     Reason = "stalled"
	Bound       Reason = "bound"
	TimedOut    Reason = "timeout"
	Interrupted Reason = "interrupted"
)

// Ending is a chain's own record of how it stopped: which of the five it was, the session
// it happened at, and the handoff a reader should open.
//
// Beside `sessions.jsonl` rather than inside it: a row describes a session, and the ending
// belongs to the chain. `-continue` and `-resume` overwrite it, which is correct — a
// resumed chain has a new ending, and the old one is a session row away.
type Ending struct {
	Reason  Reason `json:"reason"`
	Session int    `json:"session"`
	Handoff string `json:"handoff"`
	// What this run was budgeted differently from its last session, if anything. Recorded
	// and not only narrated: the terminal a background chain was resumed from is the one
	// place its warning cannot be read back from, which is the whole reason the ending is
	// a file.
	Budget []string `json:"budget_changed,omitempty"`
	// The sessions told that the plan they inherited had not been landing. Recorded because
	// a nudge nobody can read back cannot be judged against the session that followed it.
	Nudged []int `json:"nudged_sessions,omitempty"`
}

// WriteEnding records how a chain stopped.
func WriteEnding(dir string, e Ending) error {
	body, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, EndingName), body, 0o644)
}

// ReadEnding returns how a chain stopped, and whether it recorded one at all. A chain that
// is still running has none, and so has one that ran before this was written.
func ReadEnding(dir string) (Ending, bool) {
	body, err := os.ReadFile(filepath.Join(dir, EndingName))
	if err != nil {
		return Ending{}, false
	}
	var e Ending
	if err := json.Unmarshal(body, &e); err != nil {
		return Ending{}, false
	}
	return e, e.Reason != ""
}

// ClearEnding forgets the ending of the run before this one. A chain that is running has
// no ending, and a resumed chain that dies before it writes its own must not be read back
// as the run it continued.
func ClearEnding(dir string) {
	_ = os.Remove(filepath.Join(dir, EndingName))
}
