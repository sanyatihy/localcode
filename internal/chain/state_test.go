package chain

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

// The harness runs one turn's tool calls concurrently, so several gates decide at once. A
// read-modify-write loses counts there: measured, eleven permitted calls left a counter
// reading eight, and a budget that undercounts is not one.
func TestBumpCountsEveryConcurrentCaller(t *testing.T) {
	dir := t.TempDir()
	const n = 64
	seen := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			seen[i] = Bump(dir, CallsFile("s1"))
		}()
	}
	wg.Wait()
	if got := Counter(dir, CallsFile("s1")); got != n {
		t.Fatalf("%d concurrent calls counted %d", n, got)
	}
	// And each caller took a different number, which is what lets each decide for itself.
	taken := map[int]bool{}
	for _, v := range seen {
		if taken[v] {
			t.Fatalf("two callers were given the same place in the queue: %d", v)
		}
		taken[v] = true
	}
}

// A chain is read back from its layout rather than from a counter beside it, so the two
// cannot disagree about how far it got.
func TestChainLayoutIsWhatSaysHowFarAChainGot(t *testing.T) {
	state := t.TempDir()
	dir := filepath.Join(state, "chains", "20260822-120000")
	for _, n := range []string{"01", "02", "03"} {
		if err := os.MkdirAll(filepath.Join(dir, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(n, body string) {
		if err := os.WriteFile(filepath.Join(dir, n, HandoffName), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("01", "first")
	write("02", "second")
	// The third session wrote nothing, so what the next one inherits is the second's.
	if got := NextSession(dir); got != 4 {
		t.Fatalf("next session: got %d want 4", got)
	}
	if got := LatestHandoff(dir); got != filepath.Join(dir, "02", HandoffName) {
		t.Fatalf("latest handoff: got %s", got)
	}
	ids, err := Chains(state)
	if err != nil || len(ids) != 1 || ids[0] != "20260822-120000" {
		t.Fatalf("chains: %v %v", ids, err)
	}
}

// A session id names the counter, and it arrives from the harness rather than from a
// person. One that walked out of the directory would count against another session.
func TestCounterNamesCannotLeaveTheSessionDirectory(t *testing.T) {
	if got := CallsFile("../../etc/passwd"); got != "calls-passwd" {
		t.Fatalf("got %s", got)
	}
}

// Two chains started in one second are `…-150405` and `…-150405-2`, and as strings `-10`
// sorts before `-2`. Past nine of them `-continue` took the wrong chain.
func TestChainsOrderTheDisambiguatingSuffixAsANumber(t *testing.T) {
	state := t.TempDir()
	ids := []string{
		"20260827-150405", "20260827-150405-2", "20260827-150405-10",
		"20260827-150406", "20260826-090000",
	}
	for _, id := range ids {
		if err := os.MkdirAll(filepath.Join(state, "chains", id), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Chains(state)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"20260827-150406", "20260827-150405-10", "20260827-150405-2", "20260827-150405",
		"20260826-090000",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("newest first:\n got %v\nwant %v", got, want)
	}
}
