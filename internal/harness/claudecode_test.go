package harness

import (
	"encoding/json"
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

	env, err := envFromFile(writeEnv(t, "ANTHROPIC_BASE_URL=\"http://127.0.0.1:8081\"\n"))
	if err != nil {
		t.Fatalf("envFromFile: %v", err)
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

// The SessionStart hook is what puts the previous session's working state in front of the
// next one, and no Go code reads it: the committed document is asserted here so a rename
// or a malformed edit fails the offline gate rather than a session.
func TestTheCommittedHooksRunTheSessionStartScript(t *testing.T) {
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
	entries := settings.Hooks["SessionStart"]
	if len(entries) != 1 || len(entries[0].Hooks) != 1 {
		t.Fatalf("SessionStart no longer names exactly one command: %+v", entries)
	}
	hook := entries[0].Hooks[0]
	if hook.Type != "command" {
		t.Errorf("hook type is %q, not a command", hook.Type)
	}
	// $CLAUDE_PROJECT_DIR is what lets one committed path run in every worktree, and a
	// feature is always worked in one — an absolute path would run only where it was
	// written, and a bare relative path only from wherever the session was launched.
	rest, ok := strings.CutPrefix(hook.Command, "$CLAUDE_PROJECT_DIR/")
	if !ok {
		t.Fatalf("command %q is not resolved against $CLAUDE_PROJECT_DIR", hook.Command)
	}
	info, err := os.Stat(filepath.Join("../..", rest))
	if err != nil {
		t.Fatalf("the hook command is not in the repository: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("%s is not executable, so Claude Code cannot run it", rest)
	}
}

// Stdout is the hook's whole channel: Claude Code adds it to the session's context, and
// sends stderr to a debug log nothing reads. Both branches are asserted on what they
// print, because a hook that printed nothing would hand the next session nothing and say
// so nowhere.
func TestTheSessionStartHookPrintsTheHandoffOrTheShapeOfOne(t *testing.T) {
	script, err := filepath.Abs("../../harness/claude-code/hooks/session-start.sh")
	if err != nil {
		t.Fatal(err)
	}
	run := func(root string) string {
		t.Helper()
		cmd := exec.Command(script)
		cmd.Env = append(os.Environ(), "CLAUDE_PROJECT_DIR="+root)
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
	// The instruction is what keeps the file current, so it is printed whether or not
	// there is a handoff to print with it.
	if !strings.Contains(got, "Keep HANDOFF.md current") {
		t.Errorf("the instruction to keep it current was dropped:\n%s", got)
	}
}
