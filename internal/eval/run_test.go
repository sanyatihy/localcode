package eval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These exercise the public boundary — Client.Run against a fake endpoint — rather
// than the unexported checker alone. A real model cannot be asked to produce
// malformed JSON on demand, so the failure paths that matter most are only
// reachable with a double.
func fakeServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/props", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"model_path":"/tmp/fake.gguf","default_generation_settings":{"n_ctx":4096}}`))
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func toolReply(name, args string) string {
	call, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{
			"finish_reason": "tool_calls",
			"message": map[string]any{
				"tool_calls": []any{map[string]any{
					"function": map[string]any{"name": name, "arguments": args},
				}},
			},
		}},
	})
	return string(call)
}

func toolTask() *Task {
	return &Task{
		ID: "fixture", Kind: "toolcall", MaxTokens: 64,
		Messages: []Message{{Role: "user", Content: "go"}},
		Tools:    []Tool{{Type: "function", Function: ToolFunction{Name: "read_file"}}},
		Expect: Expect{
			Tool: "read_file", RequiredArgs: []string{"path"},
			ArgContains: map[string]string{"path": "session.go"},
		},
	}
}

func TestRunToolCallOutcomes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want Outcome
	}{
		{"valid call", toolReply("read_file", `{"path":"src/session.go"}`), Pass},
		{"wrong tool", toolReply("edit_file", `{"path":"src/session.go"}`), FailWrongTool},
		{"unparseable arguments", toolReply("read_file", `{"path":`), FailBadJSON},
		{"missing required arg", toolReply("read_file", `{"line":2}`), FailArgs},
		{"prose instead of a call", `{"choices":[{"message":{"content":"I would read it."}}]}`, FailNoCall},
		{"server error", `{"error":{"message":"context size exceeded"}}`, FailServer},
		{"no choices", `{"choices":[]}`, FailServer},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakeServer(t, tc.body)
			c := NewClient(srv.URL, 5*time.Second)
			res, err := c.Run(context.Background(), toolTask(), Sampling{}, nil, "")
			if err != nil {
				t.Fatalf("unexpected transport error: %v", err)
			}
			if res.Outcome != tc.want {
				t.Errorf("outcome = %s (%s), want %s", res.Outcome, res.Detail, tc.want)
			}
		})
	}
}

func TestRunRetrievalOutcomes(t *testing.T) {
	task := &Task{
		ID: "r", Kind: "retrieval", MaxTokens: 16,
		Messages:  []Message{{Role: "user", Content: "{{BODY}}"}},
		Retrieval: &Retrieval{DepthTokens: 200, Position: 0.5, Sentinel: "KEY-XYZ"},
	}
	for _, tc := range []struct {
		name, content string
		want          Outcome
	}{
		{"recalled", "The key is KEY-XYZ.", Pass},
		{"not recalled", "The key is KEY-OTHER.", FailRetrieval},
		{"empty", "   ", FailEmpty},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{
				"choices": []any{map[string]any{"message": map[string]any{"content": tc.content}}},
			})
			srv := fakeServer(t, string(body))
			c := NewClient(srv.URL, 5*time.Second)
			res, err := c.Run(context.Background(), task, Sampling{}, nil, "")
			if err != nil {
				t.Fatalf("unexpected transport error: %v", err)
			}
			if res.Outcome != tc.want {
				t.Errorf("outcome = %s (%s), want %s", res.Outcome, res.Detail, tc.want)
			}
		})
	}
}

// The haystack must be byte-identical across runs, or configs get compared against
// different prompts and the sweep measures noise.
func TestHaystackIsDeterministic(t *testing.T) {
	// Two independently constructed but equal inputs, not one expression compared to
	// itself: the property that matters is that equal inputs give equal prompts across
	// separate runs, which is what lets two configs be compared at all.
	a := buildHaystack(Retrieval{DepthTokens: 500, Position: 0.5, Sentinel: "KEY-1"})
	b := buildHaystack(Retrieval{DepthTokens: 500, Position: 0.5, Sentinel: "KEY-1"})
	if a != b {
		t.Fatal("haystack differs between builds for equal input")
	}
	if strings.Count(a, "KEY-1") != 1 {
		t.Errorf("sentinel appears %d times, want exactly 1", strings.Count(a, "KEY-1"))
	}
	if c := buildHaystack(Retrieval{DepthTokens: 900, Position: 0.5, Sentinel: "KEY-1"}); len(c) <= len(a) {
		t.Errorf("deeper haystack is not larger: %d vs %d", len(c), len(a))
	}
}

func TestNewRowUsesInjectedClock(t *testing.T) {
	orig := Now
	t.Cleanup(func() { Now = orig })
	Now = func() time.Time { return time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC) }

	row := NewRow("cfg", 0, "off", "", Sampling{}, ServerProps{NCtx: 4096}, "toolcall", Result{TaskID: "t"})
	if row.RunAt != "2026-08-17T12:00:00Z" {
		t.Errorf("RunAt = %q, want the injected time", row.RunAt)
	}
}

func TestAppendRowRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "results.jsonl")
	want := NewRow("cfg", 1, "on", "low", Sampling{}, ServerProps{NCtx: 32768}, "patch", Result{TaskID: "p", Outcome: Pass})
	if err := AppendRow(path, want); err != nil {
		t.Fatalf("AppendRow: %v", err)
	}
	if err := AppendRow(path, want); err != nil {
		t.Fatalf("AppendRow (second): %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var got Row
	if err := json.Unmarshal([]byte(splitFirstLine(string(data))), &got); err != nil {
		t.Fatalf("row is not valid JSON: %v", err)
	}
	if got.TaskID != "p" || got.ServedNCtx != 32768 {
		t.Errorf("round-tripped row = %+v", got)
	}
}

func splitFirstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}

// A capped reply must be reported as truncated, never as a wrong answer. Thinking mode
// spends the same budget on reasoning before answering, so scoring truncation as a
// quality failure silently penalises it — which is exactly what happened before this
// existed, producing eight failures that were all budget and none of them quality.
func TestTruncationIsNotScoredAsWrong(t *testing.T) {
	body := `{"choices":[{"finish_reason":"length","message":{"content":"","reasoning_content":"thinking and thinking"}}]}`
	srv := fakeServer(t, body)
	c := NewClient(srv.URL, 5*time.Second)
	for _, task := range []*Task{
		toolTask(),
		{ID: "r", Kind: "retrieval", MaxTokens: 8,
			Messages:  []Message{{Role: "user", Content: "{{BODY}}"}},
			Retrieval: &Retrieval{DepthTokens: 200, Sentinel: "KEY-X"}},
	} {
		res, err := c.Run(context.Background(), task, Sampling{}, nil, "")
		if err != nil {
			t.Fatalf("%s: %v", task.Kind, err)
		}
		if res.Outcome != FailTruncated {
			t.Errorf("%s: outcome = %s, want %s", task.Kind, res.Outcome, FailTruncated)
		}
	}
}
