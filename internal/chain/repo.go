package chain

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os/exec"
	"strconv"
	"strings"
)

// Snapshot is what a repository looked like at one moment: how many objects its database
// holds, and what its working tree carries. Known is false for a directory git cannot
// answer for, which is not the same as a repository that stood still.
type Snapshot struct {
	Objects int
	Tree    string
	Known   bool
}

// Repo reads one.
//
// Objects rather than `HEAD`, because a chain may work in a linked worktree and leave the
// checkout's `HEAD` where it was — a commit made in any of them lands in the object
// database they all share. Reading adds no objects, so a session that only looked has
// moved nothing.
func Repo(dir string) Snapshot {
	n, ok := objects(dir)
	if !ok {
		return Snapshot{}
	}
	return Snapshot{Objects: n, Tree: tree(dir), Known: true}
}

// Moved reports whether the repository changed between two readings, and whether that
// could be told at all. Unknown is neither movement nor stillness: there is no repository
// to judge by, and what the session handed on is all a caller has.
func (s Snapshot) Moved(before Snapshot) (moved, known bool) {
	if !s.Known || !before.Known {
		return false, false
	}
	return s.Objects != before.Objects || s.Tree != before.Tree, true
}

// objects counts everything git holds, loose and packed. Both, because a repack moves
// objects from one to the other without anything having been done: counting only the loose
// ones would read a `gc` as a session's work undone.
func objects(dir string) (int, bool) {
	out, err := git(dir, "count-objects", "-v")
	if err != nil {
		return 0, false
	}
	total, seen := 0, false
	scan := bufio.NewScanner(strings.NewReader(out))
	for scan.Scan() {
		field, value, ok := strings.Cut(scan.Text(), ": ")
		if !ok || (field != "count" && field != "in-pack") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return 0, false
		}
		total, seen = total+n, true
	}
	return total, seen
}

// tree is the working tree's own state, hashed because it is compared and never read. It
// carries untracked files too: a session that wrote a file and did not commit it has
// changed the repository it was given.
func tree(dir string) string {
	out, err := git(dir, "status", "--porcelain")
	if err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(out))
	return hex.EncodeToString(sum[:8])
}

// git runs one read-only command in a repository. Errors are the caller's cue that there
// is no repository here, not something to report: a chain is allowed to run outside
// version control and this test simply abstains.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	return string(out), err
}
