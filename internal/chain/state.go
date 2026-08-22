package chain

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"

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

// BatchFile counts one turn's calls. Keyed by the context the gate measured, because that
// is what does not change while a turn's calls run: a new reading means a new turn.
func BatchFile(session string, peak int) string {
	return fmt.Sprintf("batch-%s-%d", safe(session), peak)
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

// Read returns a handoff's contents, or nil when there is none.
func Read(path string) []byte {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return body
}

// Archive files a session's handoff under the chain that produced it, so a chain that goes
// wrong can be read back one session at a time rather than through its last file only.
func Archive(dir string, session int, body []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("%02d.md", session))
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("archive the handoff: %w", err)
	}
	return nil
}

// Chains lists a repository's chains, newest first. The id is a timestamp, so sorting the
// names is sorting the runs — which is what `-continue` needs and what `localcode
// sessions` prints.
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
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	return ids, nil
}

// NextSession is the number the next session in a chain takes. Sessions are numbered
// directories, so the chain's own layout is what answers this rather than a counter that
// could disagree with it.
func NextSession(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1
	}
	highest := 0
	for _, e := range entries {
		n, err := strconv.Atoi(e.Name())
		if err == nil && e.IsDir() && n > highest {
			highest = n
		}
	}
	return highest + 1
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
