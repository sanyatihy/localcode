package chain

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// run is a git command a test needs to have worked. Identity on the command line rather
// than in the environment, so a machine with no git config still runs these.
func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t",
		"-c", "commit.gpgsign=false"}, args...)
	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// repoWithACommit is the starting point every case here moves away from.
func repoWithACommit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "kept.go"), []byte("package kept\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-qm", "first")
	return dir
}

func TestACommitIsMovement(t *testing.T) {
	dir := repoWithACommit(t)
	before := Repo(dir)
	if err := os.WriteFile(filepath.Join(dir, "fixed.go"), []byte("package fixed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-qm", "the session's work")

	moved, known := Repo(dir).Moved(before)
	if !known || !moved {
		t.Fatalf("a commit must read as movement: moved %v known %v", moved, known)
	}
}

// The case the design names: a chain that works in a claimed worktree leaves the
// checkout's HEAD where it was, and only the object database they share says otherwise.
func TestACommitInAWorktreeIsMovementInTheCheckout(t *testing.T) {
	dir := repoWithACommit(t)
	before := Repo(dir)
	tree := filepath.Join(t.TempDir(), "claimed")
	run(t, dir, "worktree", "add", "-q", "-b", "claimed", tree)
	if err := os.WriteFile(filepath.Join(tree, "fixed.go"), []byte("package fixed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, tree, "add", ".")
	run(t, tree, "commit", "-qm", "work in the worktree")

	moved, known := Repo(dir).Moved(before)
	if !known || !moved {
		t.Fatalf("a commit in a worktree must read as movement in the checkout: moved %v known %v",
			moved, known)
	}
}

// A file the session wrote and did not commit is still a repository that moved: the work
// is there to be reviewed, and demanding a commit would stop a chain mid-change.
func TestAnUncommittedFileIsMovement(t *testing.T) {
	dir := repoWithACommit(t)
	before := Repo(dir)
	if err := os.WriteFile(filepath.Join(dir, "scratch.go"), []byte("package scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	moved, known := Repo(dir).Moved(before)
	if !known || !moved {
		t.Fatalf("an untracked file must read as movement: moved %v known %v", moved, known)
	}
}

// The whole point: a session that read and changed nothing has not moved the repository,
// and two of those are what the chain has to stop on.
func TestReadingMovesNothing(t *testing.T) {
	dir := repoWithACommit(t)
	before := Repo(dir)
	if _, err := os.ReadFile(filepath.Join(dir, "kept.go")); err != nil {
		t.Fatal(err)
	}
	moved, known := Repo(dir).Moved(before)
	if !known {
		t.Fatal("a repository git can answer for must be known")
	}
	if moved {
		t.Fatal("reading a file must not read as movement")
	}
}

// Unknown is not stillness. A chain outside version control has nothing here to be judged
// by, and reporting it as unmoved would stall it on its second session.
func TestADirectoryThatIsNotARepositoryIsNotKnown(t *testing.T) {
	dir := t.TempDir()
	s := Repo(dir)
	if s.Known {
		t.Fatal("a plain directory is not a repository this can read")
	}
	if _, known := s.Moved(Repo(repoWithACommit(t))); known {
		t.Fatal("unknown at either end must stay unknown")
	}
}

// The failure this feature exists for: a project worked through `kit` edits in a linked
// worktree, so a session that writes code and ends before committing changes nothing the
// checkout's own status can see. Measured on a live chain — 44 calls, 140 lines across two
// files, and the row said the repository had not moved.
func TestAnUncommittedFileInAWorktreeIsMovement(t *testing.T) {
	dir := repoWithACommit(t)
	tree := filepath.Join(t.TempDir(), "claimed")
	run(t, dir, "worktree", "add", "-q", "-b", "claimed", tree)

	before := Repo(dir)
	if err := os.WriteFile(filepath.Join(tree, "enrich.go"), []byte("package enrich\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	moved, known := Repo(dir).Moved(before)
	if !known || !moved {
		t.Fatalf("a file written in a linked worktree must read as movement in the "+
			"checkout: moved %v known %v", moved, known)
	}
}

// Adding a worktree is the repository moving too — a branch and a checkout that were not
// there before.
func TestAddingAWorktreeIsMovement(t *testing.T) {
	dir := repoWithACommit(t)
	before := Repo(dir)
	run(t, dir, "worktree", "add", "-q", "-b", "claimed", filepath.Join(t.TempDir(), "claimed"))
	if moved, known := Repo(dir).Moved(before); !known || !moved {
		t.Fatalf("adding a worktree must read as movement: moved %v known %v", moved, known)
	}
}

// The distinction the nudge rests on: a session writing throwaway probes moves a worktree,
// and two of those read as working. What a stalled chain stops producing is commits.
func TestAScratchFileMovesTheRepositoryWithoutCommittingToIt(t *testing.T) {
	dir := repoWithACommit(t)
	before := Repo(dir)
	if err := os.WriteFile(filepath.Join(dir, "probe.go"), []byte("package probe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := Repo(dir)
	if moved, known := after.Moved(before); !known || !moved {
		t.Fatalf("a scratch file is still movement: moved %v known %v", moved, known)
	}
	if committed, known := after.Committed(before); !known || committed {
		t.Fatalf("but it is not a commit: committed %v known %v", committed, known)
	}
}

func TestACommitIsCommitted(t *testing.T) {
	dir := repoWithACommit(t)
	before := Repo(dir)
	if err := os.WriteFile(filepath.Join(dir, "fixed.go"), []byte("package fixed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-qm", "the session's work")
	if committed, known := Repo(dir).Committed(before); !known || !committed {
		t.Fatalf("a commit must read as one: committed %v known %v", committed, known)
	}
}

// The same case 0035 had to fix for movement: the work lands on a branch the checkout is
// not on, and only the refs they share say so.
func TestACommitInAWorktreeIsCommittedInTheCheckout(t *testing.T) {
	dir := repoWithACommit(t)
	tree := filepath.Join(t.TempDir(), "claimed")
	run(t, dir, "worktree", "add", "-q", "-b", "claimed", tree)
	before := Repo(dir)
	if err := os.WriteFile(filepath.Join(tree, "fixed.go"), []byte("package fixed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, tree, "add", ".")
	run(t, tree, "commit", "-qm", "work in the worktree")
	if committed, known := Repo(dir).Committed(before); !known || !committed {
		t.Fatalf("a worktree's commit must count: committed %v known %v", committed, known)
	}
}

// Adding a worktree is movement but not a commit: it makes a branch at a commit that was
// already there.
func TestAddingAWorktreeIsNotACommit(t *testing.T) {
	dir := repoWithACommit(t)
	before := Repo(dir)
	run(t, dir, "worktree", "add", "-q", "-b", "claimed", filepath.Join(t.TempDir(), "claimed"))
	if committed, known := Repo(dir).Committed(before); !known || committed {
		t.Fatalf("a new branch at an old commit is not a commit: committed %v known %v", committed, known)
	}
}

// The only blocking call in the supervisor's loop that had no bound. A git that never
// answers — an index lock, a stalled filesystem — hung the chain outside the session
// clock, so nothing in the system would have ended it.
func TestARepoReadingThatNeverAnswersIsAbandonedRatherThanWaitedOut(t *testing.T) {
	bin := t.TempDir()
	script := filepath.Join(bin, "git")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 60\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	was := gitTimeout
	gitTimeout = 50 * time.Millisecond
	t.Cleanup(func() { gitTimeout = was })

	done := make(chan Snapshot, 1)
	go func() { done <- Repo(t.TempDir()) }()
	select {
	case s := <-done:
		// Abandoned reads as no repository here, which is the abstention the design
		// already handles: a chain outside version control is not a chain that stalled.
		if s.Known {
			t.Fatal("a git that never answered must not be read as a repository")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the reading was waited out rather than bounded")
	}
}
