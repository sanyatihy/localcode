package chain

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func budgeted(t *testing.T, calls int) (dir string, spec Spec) {
	t.Helper()
	dir = t.TempDir()
	l, err := NewLimits(45056, 4096, 50, calls)
	if err != nil {
		t.Fatal(err)
	}
	spec = Spec{Limits: l, Handoff: filepath.Join(dir, HandoffName)}
	if err := WriteSpec(dir, spec); err != nil {
		t.Fatal(err)
	}
	return dir, spec
}

func call(tool, path string) string {
	if tool == "Bash" {
		return `{"session_id":"s1","tool_name":"Bash","tool_input":{"command":"go test ./..."}}`
	}
	return `{"session_id":"s1","tool_name":"Write","tool_input":{"file_path":"` + path + `"}}`
}

// A call the turn's bound held back is not a call the session chose to spend, so it must
// not come out of the budget. Charged, a session batching its work would lose most of it
// to a throttle it was never told about.
func TestAThrottledCallCostsTheSessionNothing(t *testing.T) {
	dir, spec := budgeted(t, 8)
	for range spec.Limits.Batch {
		if v, err := Hook("gate", strings.NewReader(call("Bash", "")), dir); err != nil || v.Deny {
			t.Fatalf("a call under the turn's bound must be permitted: %+v %v", v, err)
		}
	}
	v, err := Hook("gate", strings.NewReader(call("Bash", "")), dir)
	if err != nil || !v.Deny {
		t.Fatalf("the turn's bound must refuse: %+v %v", v, err)
	}
	if got := Counter(dir, CallsFile("s1")); got != spec.Limits.Batch {
		t.Fatalf("the session was charged %d for %d calls it made", got, spec.Limits.Batch)
	}
}

// Every hook the harness starts for one turn decides at once. Without a race-free count
// the budget silently admits more than it says.
func TestConcurrentCallsInOneTurnAreCountedOnce(t *testing.T) {
	dir, spec := budgeted(t, 100)
	var wg sync.WaitGroup
	permitted := make([]bool, 32)
	for i := range permitted {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := Hook("gate", strings.NewReader(call("Bash", "")), dir)
			permitted[i] = err == nil && !v.Deny
		}()
	}
	wg.Wait()
	got := 0
	for _, ok := range permitted {
		if ok {
			got++
		}
	}
	if got != spec.Limits.Batch {
		t.Fatalf("32 calls in one turn: %d permitted, want the turn's bound of %d",
			got, spec.Limits.Batch)
	}
}

// The Stop hook refuses, counts its refusals, and relents rather than wedging a run.
func TestStopHookRefusesTwiceAndThenRelents(t *testing.T) {
	dir, _ := budgeted(t, 8)
	payload := `{"session_id":"s1"}`
	for i := 1; i <= stopTries; i++ {
		v, err := Hook("stop", strings.NewReader(payload), dir)
		if err != nil || !v.Deny {
			t.Fatalf("refusal %d: %+v %v", i, v, err)
		}
	}
	if v, err := Hook("stop", strings.NewReader(payload), dir); err != nil || v.Deny {
		t.Fatalf("after its refusals the hook must relent: %+v %v", v, err)
	}
}

// A harness that keeps its own context is gated on the number it reports. Without this
// the gate reads a transcript that is not there, takes -1 for the context, and holds a
// session to nothing but its call budget — which is the bound that is meant to be the
// backstop.
func TestAPayloadCarryingItsOwnContextIsGatedOnIt(t *testing.T) {
	dir, spec := budgeted(t, 100)
	payload := func(peak int) string {
		return fmt.Sprintf(`{"session_id":"pi","tool_name":"Bash","peak_tokens":%d,`+
			`"tool_input":{"command":"go test ./..."}}`, peak)
	}
	if v, err := Hook("gate", strings.NewReader(payload(spec.Limits.Ceiling-1)), dir); err != nil || v.Deny {
		t.Fatalf("a session under the ceiling must be permitted: %+v %v", v, err)
	}
	v, err := Hook("gate", strings.NewReader(payload(spec.Limits.Ceiling)), dir)
	if err != nil || !v.Deny {
		t.Fatalf("a session at the ceiling must be refused: %+v %v", v, err)
	}
	if !strings.Contains(v.Reason, fmt.Sprint(spec.Limits.Ceiling)) {
		t.Errorf("the refusal does not name the ceiling it enforced: %q", v.Reason)
	}
}
