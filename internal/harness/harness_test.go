package harness

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/sanyatihy/localcode/internal/eval"
)

// The offline condition is applied where every adapter passes through, so no harness can
// be scored offline by an adapter that forgot to apply it.
func TestSandboxedWrapsOnlyWhenARunAsksForIt(t *testing.T) {
	online := eval.Run{Workdir: "/w"}
	name, args := sandboxed(online, "pi", []string{"-p", "fix it"})
	if name != "pi" || !slices.Equal(args, []string{"-p", "fix it"}) {
		t.Errorf("an online run was changed: %s %v", name, args)
	}

	offline := eval.Run{Workdir: "/w", SandboxProfile: "/repo/harness/offline.sb"}
	name, args = sandboxed(offline, "pi", []string{"-p", "fix it"})
	if name != "sandbox-exec" {
		t.Errorf("offline run executes %q, not the sandbox", name)
	}
	want := []string{"-f", "/repo/harness/offline.sb", "pi", "-p", "fix it"}
	if !slices.Equal(args, want) {
		t.Errorf("args = %v, want %v", args, want)
	}
}

// The profile is the whole condition, so what it denies is asserted rather than trusted to
// stay as written: a profile that stopped denying the network would score every harness as
// offline-capable and read as a finding.
func TestTheCommittedOfflineProfileDeniesTheNetworkButKeepsLoopback(t *testing.T) {
	b, err := os.ReadFile("../../harness/offline.sb")
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if !strings.Contains(body, "(deny network*)") {
		t.Error("the profile no longer denies the network")
	}
	// Without loopback the model itself is unreachable, and every harness would fail for
	// a reason that has nothing to do with being offline.
	if !strings.Contains(body, "(local ip)") || !strings.Contains(body, "localhost:*") {
		t.Error("the profile no longer allows the loopback the model is served on")
	}
}
