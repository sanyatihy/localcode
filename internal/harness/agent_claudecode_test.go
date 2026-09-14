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

// onPath puts an executable of that name where the adapter will look it up, so one can be
// built without the real agent being installed on the machine running the tests.
func onPath(t *testing.T, name string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// claudeCodeCheckout writes the committed environment file, keeping the loopback default
// it carries: the point of the test below is that the launcher's endpoint replaces it.
func claudeCodeCheckout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "harness", "claude-code")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	env := `ANTHROPIC_BASE_URL="http://127.0.0.1:8081"` + "\n" +
		`ANTHROPIC_AUTH_TOKEN="local"` + "\n" +
		`CLAUDE_CODE_MAX_CONTEXT_TOKENS="45056"` + "\n" +
		`CLAUDE_CODE_MAX_OUTPUT_TOKENS="4096"` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "claude-code.env"), []byte(env), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// The endpoint the launcher hands over is the one every model call goes to. Asserted on a
// URL the committed file does not carry: one entry, the launcher's, and no other variable
// naming a host — `count_tokens` follows the base URL, so a second one would send the count
// somewhere the conversation did not go.
func TestTheSessionTalksToTheEndpointTheLauncherVerified(t *testing.T) {
	root := claudeCodeCheckout(t)
	onPath(t, "claude")
	// The port a remote endpoint is proxied onto is not the one the file names.
	const remote = "http://127.0.0.1:9099"

	a, err := newClaudeCodeAgent(root, remote)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Prepare(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	_, _, env, err := a.Command(Session{StateDir: t.TempDir(), StableDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	var base []string
	for _, kv := range env {
		name, value, _ := strings.Cut(kv, "=")
		if !isAgentVar(name) {
			continue // the machine's own environment, which this adapter does not decide
		}
		if name == "ANTHROPIC_BASE_URL" {
			base = append(base, value)
			continue
		}
		if strings.Contains(value, "127.0.0.1") || strings.Contains(value, "localhost") {
			t.Errorf("%s names a host of its own, so not every call follows the base URL", kv)
		}
	}
	if len(base) != 1 || base[0] != remote {
		t.Fatalf("the session must be given what the launcher passed, exactly once, got %q", base)
	}
}
