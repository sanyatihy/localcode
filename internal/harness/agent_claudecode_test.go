package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// settingsOf runs Prepare and hands back what it wrote. The adapter is built by hand
// rather than through NewAgent: what the hooks say does not depend on there being a
// `claude` on PATH, and a test that needed one would be testing the machine.
func settingsOf(t *testing.T) []byte {
	t.Helper()
	c := &claudeCodeAgent{root: t.TempDir()}
	dir := t.TempDir()
	if err := c.Prepare(dir); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// The gate is this binary, not a fourth shell script, so what enforces the budget is
// covered by the same `make check` as everything else that decides something.
func TestSettingsInstallTheGateAsThisBinary(t *testing.T) {
	body := settingsOf(t)
	var doc struct {
		Hooks map[string][]struct {
			Hooks []struct{ Command string } `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	pre, ok := doc.Hooks["PreToolUse"]
	if !ok || len(pre) == 0 || len(pre[0].Hooks) == 0 {
		t.Fatalf("no PreToolUse hook is installed:\n%s", body)
	}
	if !strings.HasSuffix(pre[0].Hooks[0].Command, " hook gate") {
		t.Fatalf("the gate must be this binary run as a hook: %q", pre[0].Hooks[0].Command)
	}
}

// Stop is installed for every session, and the hook reads the spec to decide which of them
// it may refuse. Installed unconditionally so that one thing decides it: an adapter that
// has to remember is an adapter that can forget, and one of the two did.
func TestStopIsInstalledForEverySessionAndTheHookDecides(t *testing.T) {
	var doc struct {
		Hooks map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal(settingsOf(t), &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.Hooks["Stop"]; !ok {
		t.Fatal("a session answering one instruction must not be able to end without a handoff")
	}
}

// An installation under a path with a space in it would otherwise run its first word.
func TestShellQuoteSurvivesAPathAShellWouldSplit(t *testing.T) {
	if got := shellQuote("/Users/a b/bin/localcode"); got != "'/Users/a b/bin/localcode'" {
		t.Fatalf("got %s", got)
	}
	if got := shellQuote("/it's/here"); got != `'/it'\''s/here'` {
		t.Fatalf("a quote in the path must not end the quoting: %s", got)
	}
}

// The child has to be told the same number, and told it once: two entries for one name
// leave which of them Claude Code reads to the C library.
func TestTheDeclarationTheChildIsGivenIsTheServedOne(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"replaced", "A=1\nCLAUDE_CODE_MAX_CONTEXT_TOKENS=45056\nB=2", "A=1\nCLAUDE_CODE_MAX_CONTEXT_TOKENS=28672\nB=2"},
		{"appended", "A=1", "A=1\nCLAUDE_CODE_MAX_CONTEXT_TOKENS=28672"},
		{"deduped", "CLAUDE_CODE_MAX_CONTEXT_TOKENS=1\nCLAUDE_CODE_MAX_CONTEXT_TOKENS=2", "CLAUDE_CODE_MAX_CONTEXT_TOKENS=28672"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := setEnv(strings.Split(tc.in, "\n"), "CLAUDE_CODE_MAX_CONTEXT_TOKENS", "28672")
			if strings.Join(got, "\n") != tc.want {
				t.Fatalf("got %q, want %q", strings.Join(got, "\n"), tc.want)
			}
		})
	}
}

// A session that has not been prepared has no settings file, so it would run with no gate
// at all. Refused rather than run: an unbounded session is the failure the gate exists to
// prevent, and it is silent.
func TestASessionRefusesToStartWithoutItsGate(t *testing.T) {
	c := &claudeCodeAgent{root: t.TempDir()}
	if _, _, _, err := c.Command(Session{Goal: "do the thing"}); err == nil {
		t.Fatal("a session with no settings file must be refused")
	}
}
