package chain

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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
