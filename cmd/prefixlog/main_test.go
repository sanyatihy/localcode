package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runLog(t *testing.T, args ...string) (string, error) {
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

// One server log with one complete request, in the shape llama-server prints.
func serverLog(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "server.log")
	// The shape internal/prefix parses, taken from that package's own fixture rather than
	// invented here: prompt 2757 = n_tokens 2760 + Offset 1 - 4 generated, of which 826
	// were ingested.
	body := `0.42.617.733 I slot launch_slot_: id  0 | task 8 | processing task, is_child = 0
0.51.025.995 I slot print_timing: id  0 | task 8 | prompt eval time =    8149.65 ms /   826 tokens (    9.87 ms per token,   101.35 tokens per second)
0.51.026.007 I slot print_timing: id  0 | task 8 |        eval time =     258.60 ms /     4 tokens (   86.20 ms per token,    11.60 tokens per second)
0.51.026.123 I slot      release: id  0 | task 8 | stop processing: n_tokens = 2760, truncated = 0
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRefusesWhatItCannotRead(t *testing.T) {
	if _, err := runLog(t); err == nil {
		t.Error("-log is required and its absence must be refused")
	}
	if _, err := runLog(t, "-log", filepath.Join(t.TempDir(), "nope.log")); err == nil {
		t.Error("a log that is not there must be refused")
	}
}

// A log carrying no completed request is refused rather than summarised as zero: zero
// requests and a log this build cannot parse look identical in a total.
func TestRefusesALogWithNoCompletedRequest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.log")
	if err := os.WriteFile(path, []byte("ggml_metal_init: found device\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runLog(t, "-log", path); err == nil {
		t.Error("a log with no completed request must be refused, not reported as none")
	}
}

func TestReadsARequestAndReportsWhatItIngested(t *testing.T) {
	out, err := runLog(t, "-log", serverLog(t))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out, "1 requests") {
		t.Errorf("expected one request in:\n%s", out)
	}
}

// -check is the whole reason the derived prompt figure can be trusted, so a disagreement has
// to exit 1 and print every one — distinct from exit 2, which is "could not be carried out".
func TestCheckDisagreementIsItsOwnExitCode(t *testing.T) {
	rows := filepath.Join(t.TempDir(), "rows.jsonl")
	if err := os.WriteFile(rows, []byte(`{"prompt_tokens":1,"ingested_tokens":1,"cached_tokens":0}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runLog(t, "-log", serverLog(t), "-check", rows)
	if !errors.Is(err, errDisagrees) {
		t.Fatalf("a disagreement must return errDisagrees, got %v", err)
	}
	if !strings.Contains(out, "the endpoint reported") {
		t.Errorf("every disagreement must be printed, got:\n%s", out)
	}
}

func TestCheckAgreementIsSilentSuccess(t *testing.T) {
	rows := filepath.Join(t.TempDir(), "rows.jsonl")
	if err := os.WriteFile(rows, []byte(`{"prompt_tokens":2757,"ingested_tokens":826,"cached_tokens":1931}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runLog(t, "-log", serverLog(t), "-check", rows)
	if err != nil {
		t.Fatalf("agreeing accounts must not error: %v\n%s", err, out)
	}
	if !strings.Contains(out, "they agree") {
		t.Errorf("expected agreement to be stated, got:\n%s", out)
	}
}
