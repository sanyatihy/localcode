package harness

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeEnv(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "claude-code.env")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseEnvFileReadsTheCommittedForm(t *testing.T) {
	p := writeEnv(t, "# a comment\n\nANTHROPIC_BASE_URL=\"http://127.0.0.1:8081\"\nCLAUDE_CODE_DISABLE_CRON='1'\nBARE=2\n")
	got, err := parseEnvFile(p)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []string{"ANTHROPIC_BASE_URL=http://127.0.0.1:8081", "CLAUDE_CODE_DISABLE_CRON=1", "BARE=2"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// A line that cannot be read must stop the run. Skipping it would serve a different
// configuration under the same label, which is the failure this whole repo is arranged
// against.
func TestParseEnvFileRefusesALineItCannotRead(t *testing.T) {
	if _, err := parseEnvFile(writeEnv(t, "ANTHROPIC_BASE_URL=x\nnonsense\n")); err == nil {
		t.Fatal("expected an error naming the line")
	}
	if _, err := parseEnvFile(writeEnv(t, "# only comments\n")); err == nil {
		t.Fatal("expected an error: the file sets nothing")
	}
	if _, err := parseEnvFile(filepath.Join(t.TempDir(), "absent.env")); err == nil {
		t.Fatal("expected an error: the file is not there")
	}
}

// The parent's own agent variables must not reach the child: a stray ANTHROPIC_API_KEY
// outranks the file's credential, and a session running inside Claude Code exports
// CLAUDE_CODE_* variables that change the tool set.
func TestEnvFromFileDropsInheritedAgentVariables(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "inherited")
	t.Setenv("CLAUDE_CODE_ENABLE_TASKS", "1")
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("PATH_LIKE_UNRELATED", "keep-me")

	env, err := EnvFromFile(writeEnv(t, "ANTHROPIC_BASE_URL=\"http://127.0.0.1:8081\"\n"))
	if err != nil {
		t.Fatalf("EnvFromFile: %v", err)
	}
	joined := strings.Join(env, "\n")
	for _, gone := range []string{"ANTHROPIC_API_KEY=", "CLAUDE_CODE_ENABLE_TASKS=", "CLAUDECODE="} {
		if strings.Contains(joined, gone) {
			t.Errorf("%s survived into the child environment", gone)
		}
	}
	if !strings.Contains(joined, "PATH_LIKE_UNRELATED=keep-me") {
		t.Error("an unrelated variable was dropped")
	}
	if !strings.Contains(joined, "ANTHROPIC_BASE_URL=http://127.0.0.1:8081") {
		t.Error("the file's own variable did not reach the child")
	}
}

// The committed file is what a run is produced by, so it has to parse.
func TestTheCommittedEnvironmentParses(t *testing.T) {
	vars, err := parseEnvFile("../../harness/claude-code/claude-code.env")
	if err != nil {
		t.Fatalf("committed environment: %v", err)
	}
	joined := strings.Join(vars, "\n")
	for _, want := range []string{"ANTHROPIC_BASE_URL=", "ANTHROPIC_DEFAULT_HAIKU_MODEL="} {
		if !strings.Contains(joined, want) {
			t.Errorf("committed environment no longer sets %s", want)
		}
	}
}

// No Go code reads the hooks, so a rename or a bad edit would fail a session rather than
// the gate.
func TestTheCommittedHooksRunScriptsThatAreThere(t *testing.T) {
	b, err := os.ReadFile("../../harness/claude-code/hooks.json")
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(b, &settings); err != nil {
		t.Fatalf("hooks.json does not parse: %v", err)
	}
	for _, event := range []string{"SessionStart", "PreCompact", "SessionEnd"} {
		entries := settings.Hooks[event]
		if len(entries) != 1 || len(entries[0].Hooks) != 1 {
			t.Fatalf("%s no longer names exactly one command: %+v", event, entries)
		}
		hook := entries[0].Hooks[0]
		if hook.Type != "command" {
			t.Errorf("%s hook type is %q, not a command", event, hook.Type)
		}
		// One committed path has to run in every worktree.
		rest, ok := strings.CutPrefix(hook.Command, "$CLAUDE_PROJECT_DIR/")
		if !ok {
			t.Fatalf("%s command %q is not resolved against $CLAUDE_PROJECT_DIR", event, hook.Command)
		}
		info, err := os.Stat(filepath.Join("../..", rest))
		if err != nil {
			t.Fatalf("the %s command is not in the repository: %v", event, err)
		}
		if info.Mode()&0o111 == 0 {
			t.Errorf("%s is not executable, so Claude Code cannot run it", rest)
		}
	}
}

// Stdout is the hook's whole channel, so both branches are asserted on what they print.
func TestTheSessionStartHookPrintsTheHandoffOrTheShapeOfOne(t *testing.T) {
	script, err := filepath.Abs("../../harness/claude-code/hooks/session-start.sh")
	if err != nil {
		t.Fatal(err)
	}
	run := func(root string) string {
		t.Helper()
		cmd := exec.Command(script)
		cmd.Env = append(os.Environ(), "CLAUDE_PROJECT_DIR="+root, "LOCALCODE_HANDOFF_DIR=")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("hook failed: %v: %s", err, out)
		}
		return string(out)
	}

	if got := run(t.TempDir()); !strings.Contains(got, "**Box:**") {
		t.Errorf("with nothing handed over, the hook does not say what to write:\n%s", got)
	}

	handed := t.TempDir()
	if err := os.WriteFile(filepath.Join(handed, "HANDOFF.md"), []byte("**Next:** finish the box\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := run(handed)
	if !strings.Contains(got, "**Next:** finish the box") {
		t.Errorf("the handoff was not printed:\n%s", got)
	}
	// Printed whether or not there is a handoff to print with it.
	if !strings.Contains(got, "current as you work") {
		t.Errorf("the instruction to keep it current was dropped:\n%s", got)
	}
}

// Exit 2 is the only code that blocks a compaction, and nothing the hook prints is seen —
// so the record is the only trace.
func TestThePreCompactHookRefusesAndRecordsThatItFired(t *testing.T) {
	script, err := filepath.Abs("../../harness/claude-code/hooks/pre-compact.sh")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	fire := func(trigger string) {
		t.Helper()
		cmd := exec.Command(script)
		cmd.Stdin = strings.NewReader(`{"session_id":"s1","trigger":"` + trigger + `"}`)
		cmd.Env = append(os.Environ(), "CLAUDE_PROJECT_DIR="+root)
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 {
			t.Fatalf("hook exited %v, and only 2 refuses a compaction: %s", err, out)
		}
	}

	// A refused session keeps running, so the hook fires again and the record appends.
	fire("auto")
	fire("auto")

	b, err := os.ReadFile(filepath.Join(root, "results", "precompact.jsonl"))
	if err != nil {
		t.Fatalf("nothing was recorded: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatalf("recorded %d refusals, want 2:\n%s", len(lines), b)
	}
	var fired map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &fired); err != nil {
		t.Fatalf("the record is not JSON: %v", err)
	}
	if fired["trigger"] != "auto" {
		t.Errorf("the trigger was not carried through: %v", fired["trigger"])
	}
	if fired["at"] == nil {
		t.Error("the record carries no time, so refusals cannot be placed in a session")
	}
}

// A handoff the session wrote knows what it meant to do; an extraction only knows what it
// did. So the extraction must never win.
func TestTheSessionEndHookWritesAHandoffOnlyWhenTheSessionWroteNone(t *testing.T) {
	script, err := filepath.Abs("../../harness/claude-code/hooks/session-end.sh")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	transcript := filepath.Join(root, "transcript.jsonl")
	said := func(blocks string) string {
		return `{"type":"assistant","message":{"content":[` + blocks + "]}}\n"
	}
	body := said(`{"type":"tool_use","name":"Read","input":{"file_path":"`+root+`/read.go"}}`) +
		said(`{"type":"tool_use","name":"Edit","input":{"file_path":"`+root+`/edited.go"}}`) +
		said(`{"type":"text","text":"Next: run the tests."}`)
	if err := os.WriteFile(transcript, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	end := func() {
		t.Helper()
		cmd := exec.Command(script)
		cmd.Stdin = strings.NewReader(`{"session_id":"s1","reason":"other","transcript_path":"` + transcript + `"}`)
		cmd.Env = append(os.Environ(), "CLAUDE_PROJECT_DIR="+root)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("hook failed: %v: %s", err, out)
		}
	}
	handoff := filepath.Join(root, "HANDOFF.md")

	end()
	b, err := os.ReadFile(handoff)
	if err != nil {
		t.Fatalf("no fallback was written: %v", err)
	}
	got := string(b)
	// Relative to the checkout, edited files first.
	for _, want := range []string{"`edited.go` (edited)", "`read.go`", "Next: run the tests."} {
		if !strings.Contains(got, want) {
			t.Errorf("the fallback does not carry %q:\n%s", want, got)
		}
	}
	// Read again at every session start, from a transcript full of heredocs.
	if n := strings.Count(got, "\n"); n > 40 {
		t.Errorf("the fallback is %d lines, over the 40 it is specified at:\n%s", n, got)
	}

	if err := os.WriteFile(handoff, []byte("what the session meant to do\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	end()
	b, err = os.ReadFile(handoff)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "what the session meant to do\n" {
		t.Errorf("the session's own handoff was overwritten:\n%s", b)
	}
}

// The override is what lets 0016 run outside this checkout. Both halves matter: state
// follows LOCALCODE_HANDOFF_DIR when it is set, and working on localcode itself is
// unchanged when it is not.
func TestTheHooksPutStateWhereTheOverrideSaysOrTheCheckoutOtherwise(t *testing.T) {
	end, err := filepath.Abs("../../harness/claude-code/hooks/session-end.sh")
	if err != nil {
		t.Fatal(err)
	}
	payload := `{"session_id":"s1","reason":"other","transcript_path":"/nonexistent"}`

	t.Run("override wins", func(t *testing.T) {
		project, state := t.TempDir(), filepath.Join(t.TempDir(), "elsewhere")
		cmd := exec.Command(end)
		cmd.Stdin = strings.NewReader(payload)
		cmd.Env = append(os.Environ(),
			"CLAUDE_PROJECT_DIR="+project, "LOCALCODE_HANDOFF_DIR="+state)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("hook failed: %v: %s", err, out)
		}
		if _, err := os.Stat(filepath.Join(state, "HANDOFF.md")); err != nil {
			t.Fatalf("no handoff under the override: %v", err)
		}
		// The visited repository is the thing being protected.
		if _, err := os.Stat(filepath.Join(project, "HANDOFF.md")); err == nil {
			t.Fatal("a handoff was written into the repository being visited")
		}
	})

	t.Run("checkout by default", func(t *testing.T) {
		project := t.TempDir()
		cmd := exec.Command(end)
		cmd.Stdin = strings.NewReader(payload)
		// A parent that has one set must not leak it into the unset case.
		cmd.Env = append(os.Environ(), "CLAUDE_PROJECT_DIR="+project, "LOCALCODE_HANDOFF_DIR=")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("hook failed: %v: %s", err, out)
		}
		if _, err := os.Stat(filepath.Join(project, "HANDOFF.md")); err != nil {
			t.Fatalf("working on localcode itself must still write to the checkout: %v", err)
		}
	})
}

// The shape the hook prints has to name the path it actually wants, or the model creates
// a HANDOFF.md in the repository it is standing in — which is what it did.
func TestTheSessionStartHookNamesThePathItWants(t *testing.T) {
	start, err := filepath.Abs("../../harness/claude-code/hooks/session-start.sh")
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(t.TempDir(), "elsewhere")
	cmd := exec.Command(start)
	cmd.Env = append(os.Environ(), "CLAUDE_PROJECT_DIR="+t.TempDir(), "LOCALCODE_HANDOFF_DIR="+state)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook failed: %v: %s", err, out)
	}
	if !strings.Contains(string(out), filepath.Join(state, "HANDOFF.md")) {
		t.Fatalf("the injected instruction must name the real path:\n%s", out)
	}
}
