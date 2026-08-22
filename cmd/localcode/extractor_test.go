package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sanyatihy/localcode/internal/chain"
)

// The extractor is the floor under every other mechanism here: it writes the handoff for a
// session that did not. It has been wrong twice in the same way, so it is tested from the
// outside — the script, a real transcript, and the file it leaves.
func extract(t *testing.T, cwd string, rows []map[string]any) string {
	t.Helper()
	script := filepath.Join("..", "..", "harness", "claude-code", "hooks", "session-end.sh")
	if _, err := os.Stat(script); err != nil {
		t.Skip("not run from the checkout")
	}
	state := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	f, err := os.Create(transcript)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		body, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(append(body, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	_ = f.Close()

	payload, err := json.Marshal(map[string]any{
		"session_id": "s1", "reason": "other", "transcript_path": transcript, "cwd": cwd,
	})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(script)
	cmd.Stdin = strings.NewReader(string(payload))
	cmd.Env = append(os.Environ(), "LOCALCODE_HANDOFF_DIR="+state)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("session-end.sh: %v: %s", err, out)
	}
	body, err := os.ReadFile(filepath.Join(state, chain.HandoffName))
	if err != nil {
		t.Fatalf("the extractor wrote no handoff: %v", err)
	}
	return string(body)
}

func assistant(text string, calls ...map[string]any) map[string]any {
	content := []any{}
	if text != "" {
		content = append(content, map[string]any{"type": "text", "text": text})
	}
	for _, c := range calls {
		content = append(content, c)
	}
	return map[string]any{"type": "assistant", "message": map[string]any{"content": content}}
}

func edit(path string) map[string]any {
	return map[string]any{"type": "tool_use", "name": "Edit", "input": map[string]any{"file_path": path}}
}

// An API error is rendered as an assistant message. Taken as the last thing the session
// said, it becomes the next session's plan — which is what seven of eight handoffs in the
// first measured chain carried, and what this feature exists to stop.
func TestTheExtractorNeverMakesAnApiErrorTheNextStep(t *testing.T) {
	repo := t.TempDir()
	got := extract(t, repo, []map[string]any{
		assistant("Fixing the even-length case in median.go.", edit(filepath.Join(repo, "median.go"))),
		{"type": "assistant", "isApiErrorMessage": true,
			"message": map[string]any{"content": []any{map[string]any{"type": "text", "text": "Prompt is too long"}}}},
	})
	next := chain.Next([]byte(got))
	if strings.Contains(next, "too long") {
		t.Fatalf("an error became the plan: %q", next)
	}
	if !strings.Contains(next, "median.go") && !strings.Contains(next, "even-length") {
		t.Fatalf("the last thing the session actually said is what it has to carry: %q", next)
	}
}

// A session that only ever errored still has to hand on what it did, or the next one
// starts from nothing at all.
func TestTheExtractorFallsBackToWhatTheSessionDid(t *testing.T) {
	repo := t.TempDir()
	got := extract(t, repo, []map[string]any{
		assistant("", edit(filepath.Join(repo, "clamp.go"))),
		{"type": "assistant", "isApiErrorMessage": true,
			"message": map[string]any{"content": []any{map[string]any{"type": "text", "text": "Prompt is too long"}}}},
	})
	if next := chain.Next([]byte(got)); !strings.Contains(next, "clamp.go") {
		t.Fatalf("the last thing done is the only lead there is: %q", next)
	}
}

// Shortened against the state directory every path stays absolute, and a handoff of eight
// absolute paths is mostly noise in a window this small.
func TestTheExtractorShortensPathsAgainstTheWorkingDirectory(t *testing.T) {
	repo := t.TempDir()
	got := extract(t, repo, []map[string]any{
		assistant("done", edit(filepath.Join(repo, "median.go"))),
	})
	if !strings.Contains(got, "`median.go`") {
		t.Fatalf("paths must be relative to where the next session stands:\n%s", got)
	}
}
