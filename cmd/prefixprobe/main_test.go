package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runProbe(t *testing.T, args ...string) (string, error) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	runErr := run(args, f, f)
	_ = f.Close()
	b, _ := os.ReadFile(out)
	return string(b), runErr
}

// Unlike the scorer, this refuses to run without /props. Every row it writes is a claim
// about one serving config against another, and a run that cannot name what served it
// cannot support that claim.
func TestTheProbeRefusesAnEndpointThatWillNotSayWhatItServes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening now

	_, err := runProbe(t, "-endpoint", url)
	if err == nil {
		t.Fatal("a probe that cannot name what served it must refuse")
	}
	if !strings.Contains(err.Error(), "is not evidence") {
		t.Fatalf("the refusal must say why it is one: %v", err)
	}
}

// A conversation is at least one turn, and a bound below that measures nothing. Refused
// before the endpoint is asked, since it is the flags rather than the machine that are
// wrong.
func TestTheProbeRefusesAConversationWithNoTurns(t *testing.T) {
	_, err := runProbe(t, "-turns", "0")
	if err == nil || !strings.Contains(err.Error(), "at least one turn") {
		t.Fatalf("want a refusal naming the bound, got %v", err)
	}
}

// The rows go to disk as they are taken, and the summary is over what was measured. A
// condition here is tens of minutes, so what was recorded before a failure must be on disk.
func TestTheProbeWritesEachRowAsItIsTakenAndSummarisesWhatItMeasured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/props") {
			_, _ = w.Write([]byte(`{"model_path":"/m/Qwen.gguf","default_generation_settings":{"n_ctx":8192}}`))
			return
		}
		// One turn's reply, with the accounting the probe's whole measurement is.
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn",
			"usage":{"input_tokens":40,"output_tokens":2,"cache_read_input_tokens":60}}`))
	}))
	defer srv.Close()

	results := filepath.Join(t.TempDir(), "rows.jsonl")
	out, err := runProbe(t, "-endpoint", srv.URL, "-results", results,
		"-turns", "2", "-system-words", "10", "-turn-words", "10", "-config", "probe")
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	body, err := os.ReadFile(results)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(strings.TrimSpace(string(body)), "\n") + 1; got != 2 {
		t.Fatalf("wrote %d rows for 2 turns:\n%s", got, body)
	}
	// The served config travels with the rows: a row that cannot say what served it
	// cannot be attributed to one.
	if !strings.Contains(string(body), `"served_n_ctx":8192`) {
		t.Fatalf("the rows do not name what served them:\n%s", body)
	}
	if !strings.Contains(out, "2 turns:") {
		t.Fatalf("the summary is over what was measured:\n%s", out)
	}
}
