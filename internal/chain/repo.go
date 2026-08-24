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
	Commits int
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
	return Snapshot{Objects: n, Commits: commits(dir), Tree: tree(dir), Known: true}
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

// Committed reports whether the repository gained a commit between two readings, and
// whether that could be told.
//
// A narrower question than Moved, and a different one: a session writing throwaway probes
// moves a worktree, so two of those read as working while nothing durable lands. What a
// chain talking itself into a false premise stops producing is commits — measured, five
// sessions and 2h44m of them.
func (s Snapshot) Committed(before Snapshot) (committed, known bool) {
	if !s.Known || !before.Known {
		return false, false
	}
	return s.Commits > before.Commits, true
}

// commits counts what is reachable from every ref, so a commit made on any branch counts
// and a branch made at one that was already there does not. Reachability rather than the
// refs themselves: `kit claim` creates a branch, which moves a ref without adding work.
func commits(dir string) int {
	out, err := git(dir, "rev-list", "--all", "--count")
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0
	}
	return n
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
//
// Every worktree, not the one the chain was started in. A project worked through `kit`
// edits in a linked worktree, so a session that writes code and ends before committing
// changes nothing the checkout's own status can see — measured on a live chain, 44 calls
// and 140 lines across two files read as having done nothing. The path is hashed beside
// each status, so a worktree appearing is movement as much as a file inside one.
func tree(dir string) string {
	var parts []string
	for _, w := range worktrees(dir) {
		out, err := git(w, "status", "--porcelain")
		if err != nil {
			// A worktree git lists but cannot stat is the repository's own housekeeping,
			// and a reading that refused because of one would stop a chain for something
			// the session did not do.
			continue
		}
		parts = append(parts, w, out)
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:8])
}

// worktrees names every working tree the repository has, the one asked about included.
// Falling back to that one alone, because a git too old to list them still has it.
func worktrees(dir string) []string {
	out, err := git(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return []string{dir}
	}
	var paths []string
	scan := bufio.NewScanner(strings.NewReader(out))
	for scan.Scan() {
		if p, ok := strings.CutPrefix(scan.Text(), "worktree "); ok {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return []string{dir}
	}
	return paths
}

// git runs one read-only command in a repository. Errors are the caller's cue that there
// is no repository here, not something to report: a chain is allowed to run outside
// version control and this test simply abstains.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	return string(out), err
}
