package eval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// captureRequest returns a server that records the decoded request body it was sent.
// What matters here is what went on the wire: a reasoning level that never left the
// process would leave every run measuring the model's default while the results file
// claimed otherwise, which is the failure this whole axis was added to end.
func captureRequest(t *testing.T, got *map[string]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/props", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"model_path":"/tmp/fake.gguf","default_generation_settings":{"n_ctx":4096}}`))
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(got)
		_, _ = w.Write([]byte(toolReply("read_file", `{"path":"src/auth/session.go"}`)))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestReasoningEffortReachesTheWire(t *testing.T) {
	for _, tc := range []struct {
		effort   string
		wantSent bool
	}{
		{"low", true},
		{"medium", true},
		{"xhigh", true},
		// Empty must send nothing rather than a literal "", so the model's own
		// default applies and the omission is visible as an omission.
		{"", false},
	} {
		var got map[string]any
		srv := captureRequest(t, &got)
		c := NewClient(srv.URL, 0)
		if _, err := c.Run(context.Background(), toolTask(), Sampling{}, nil, tc.effort, qwenProfile(t)); err != nil {
			t.Fatalf("effort %q: %v", tc.effort, err)
		}
		v, present := got["reasoning_effort"]
		if present != tc.wantSent {
			t.Errorf("effort %q: field present=%v, want %v (body: %v)", tc.effort, present, tc.wantSent, got)
		}
		if tc.wantSent && v != tc.effort {
			t.Errorf("effort %q: sent %v", tc.effort, v)
		}
	}
}

// The level is a separate axis from enable_thinking, not a finer version of it, so
// both must be able to travel in one request.
func TestEffortAndThinkingAreIndependent(t *testing.T) {
	var got map[string]any
	srv := captureRequest(t, &got)
	c := NewClient(srv.URL, 0)
	on := true
	if _, err := c.Run(context.Background(), toolTask(), Sampling{}, &on, "low", qwenProfile(t)); err != nil {
		t.Fatal(err)
	}
	if got["reasoning_effort"] != "low" {
		t.Errorf("reasoning_effort = %v, want low", got["reasoning_effort"])
	}
	kw, ok := got["chat_template_kwargs"].(map[string]any)
	if !ok || kw["enable_thinking"] != true {
		t.Errorf("enable_thinking did not survive alongside the effort: %v", got)
	}
}

// A row without the level cannot say what it measured, and an empty value is a
// measurement of the model's default rather than of nothing.
func TestRowCarriesReasoningEffort(t *testing.T) {
	r := NewRow("cfg", 0, "on", "low", Sampling{}, ServerProps{NCtx: 32768}, "patch", Result{TaskID: "t"})
	if r.ReasoningEffort != "low" {
		t.Errorf("row lost the effort: %+v", r)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if _, ok := back["reasoning_effort"]; !ok {
		t.Error("reasoning_effort is absent from the serialised row; results would not record the axis")
	}
}

// A task that runs forever is a bug, not a slow answer. The budget must produce a
// scored outcome and must not abort the suite: a sweep is hours long and losing the
// rest of it to one stuck task is the failure this guard exists to prevent.
func TestOverBudgetIsScoredAndDoesNotAbort(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/props", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"model_path":"/tmp/fake.gguf","default_generation_settings":{"n_ctx":4096}}`))
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		// Slower than the budget, but bounded. A handler that waits only on the request
		// context leaves httptest.Close blocking on it — client cancellation does not
		// reach an httptest handler here, so the fallback is what ends this, and it is
		// kept short because the rule against tasks that run forever applies to our own
		// tests too.
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	task := toolTask()
	task.TimeoutSeconds = 1
	res, err := NewClient(srv.URL, 0).Run(context.Background(), task, Sampling{}, nil, "", qwenProfile(t))
	if err != nil {
		t.Fatalf("the budget aborted the suite instead of scoring the task: %v", err)
	}
	if res.Outcome != FailOverBudget {
		t.Errorf("outcome = %q, want %q", res.Outcome, FailOverBudget)
	}
}

// A missing budget must not mean "unbounded".
func TestTasksGetADefaultBudget(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "t.json")
	if err := os.WriteFile(p, []byte(`{"id":"x","kind":"toolcall"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	task, err := LoadTask(p)
	if err != nil {
		t.Fatal(err)
	}
	if task.TimeoutSeconds <= 0 {
		t.Errorf("loaded task has no budget: %+v", task)
	}
}
