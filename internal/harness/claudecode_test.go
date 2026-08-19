package harness

import (
	"os"
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
