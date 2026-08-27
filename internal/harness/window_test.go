package harness

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// `Window` and the two readers under it turn what a server serves into the pair every
// bound in internal/chain is derived from. They had no test: a budget taken from a number
// nobody checked is a budget that protects against a wall that is not the one there.

// Both numbers are needed and neither is guessed. A file naming one of them would leave a
// session budgeted against a reservation nobody declared.
func TestTheDeclarationIsReadFromTheFileAndRefusedWhenItIsNotThere(t *testing.T) {
	for _, tc := range []struct {
		name        string
		env         []string
		wantContext int
		wantOutput  int
		wantErr     string
	}{
		{
			name:        "both",
			env:         []string{"CLAUDE_CODE_MAX_CONTEXT_TOKENS=45056", "CLAUDE_CODE_MAX_OUTPUT_TOKENS=4096"},
			wantContext: 45056, wantOutput: 4096,
		},
		{
			name:    "no output reservation",
			env:     []string{"CLAUDE_CODE_MAX_CONTEXT_TOKENS=45056"},
			wantErr: "must set both",
		},
		{
			name:    "no context",
			env:     []string{"CLAUDE_CODE_MAX_OUTPUT_TOKENS=4096"},
			wantErr: "must set both",
		},
		{
			name:    "not a number",
			env:     []string{"CLAUDE_CODE_MAX_CONTEXT_TOKENS=lots", "CLAUDE_CODE_MAX_OUTPUT_TOKENS=4096"},
			wantErr: "is not a number",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotContext, gotOutput, err := declared(tc.env)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want an error saying %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if gotContext != tc.wantContext || gotOutput != tc.wantOutput {
				t.Fatalf("got %d and %d, want %d and %d",
					gotContext, gotOutput, tc.wantContext, tc.wantOutput)
			}
		})
	}
}

// The child is told what the server serves, not what the file says. A file carries one
// number and `-config` chooses which context is served, so a declaration taken from the
// file is a claim about whichever server it was written for.
func TestWindowDeclaresTheServedContextAndKeepsTheFilesReservation(t *testing.T) {
	c := &claudeCodeAgent{claudeCodeRecorder: claudeCodeRecorder{env: []string{
		"CLAUDE_CODE_MAX_CONTEXT_TOKENS=45056",
		"CLAUDE_CODE_MAX_OUTPUT_TOKENS=4096",
	}}}
	declaredCtx, output, err := c.Window(22528)
	if err != nil {
		t.Fatal(err)
	}
	if declaredCtx != 22528 {
		t.Fatalf("declared %d, want the 22528 the server serves", declaredCtx)
	}
	if output != 4096 {
		t.Fatalf("reservation %d, want the file's 4096", output)
	}
	// What the child is given has to agree, and once: two entries for one name leave which
	// of them is read to the C library.
	if got := count(c.env, "CLAUDE_CODE_MAX_CONTEXT_TOKENS="); got != 1 {
		t.Fatalf("the declaration appears %d times in the child's environment", got)
	}
	if !has(c.env, "CLAUDE_CODE_MAX_CONTEXT_TOKENS="+strconv.Itoa(22528)) {
		t.Fatalf("the child was not told what the server serves: %v", c.env)
	}
	// The harness's own check holds back an undocumented fraction of whatever it is told.
	// The gate is what prevents the server's 400 here, so the check is off.
	if !has(c.env, "CLAUDE_CODE_DISABLE_UNKNOWN_MODEL_WINDOW_ENFORCEMENT=1") {
		t.Fatalf("the harness's own window check was left on: %v", c.env)
	}
}

// Pi's numbers come from the committed provider file, because that file is the declaration
// pi acts on. A second copy in Go would be a second answer to the same question.
func TestPiDeclaresIsReadFromTheProviderFile(t *testing.T) {
	t.Run("the committed one", func(t *testing.T) {
		model, maxTokens, err := piDeclares(piProviderFile(filepath.Join("..", "..")))
		if err != nil {
			t.Fatal(err)
		}
		if model == "" || maxTokens <= 0 {
			t.Fatalf("model %q, maxTokens %d — a session is budgeted from these", model, maxTokens)
		}
	})

	for _, tc := range []struct{ name, body, wantErr string }{
		{"no id", "export default { maxTokens: 4096 }", "must declare"},
		{"no maxTokens", "  id: \"local/model\"\n", "must declare"},
		{"unreadable", "", "not readable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "local-provider.js")
			if tc.body != "" {
				if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := piDeclares(path); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want an error saying %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func has(env []string, entry string) bool {
	for _, kv := range env {
		if kv == entry {
			return true
		}
	}
	return false
}

func count(env []string, prefix string) int {
	n := 0
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			n++
		}
	}
	return n
}
