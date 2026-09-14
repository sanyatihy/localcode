package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sanyatihy/localcode/internal/eval"
)

// eval's refusals are the contract a sweep script branches on, and every one of them exists
// because it was learned the expensive way. These drive `run` the way the command line does,
// and stop before any request: none of them needs a server.
func runEval(t *testing.T, args ...string) error {
	t.Helper()
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = null.Close() })
	return run(args, null, null)
}

func repo(parts ...string) string {
	return filepath.Join(append([]string{"..", ".."}, parts...)...)
}

// The refusal that cost 114 rows: a toggle carries its own sampling, so setting one without
// the other measures the pair rather than the toggle.
func TestRefusesTheToggleWithoutSampling(t *testing.T) {
	err := runEval(t, "-task", repo("tasks", "toolcall-read-file.json"), "-thinking", "off")
	if err == nil {
		t.Fatal("a thinking mode with no sampling must be refused")
	}
	if !strings.Contains(err.Error(), "sampling") {
		t.Errorf("the refusal must name sampling, or the reader cannot act on it: %v", err)
	}
}

// Each of these is enough on its own to make a run measure something its row would not say.
func TestRefusesWhatItCannotRun(t *testing.T) {
	task := repo("tasks", "toolcall-read-file.json")
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"neither -task nor -tasks", nil},
		{"both -task and -tasks", []string{"-task", task, "-tasks", repo("tasks")}},
		{"a dialect that is not a dialect", []string{"-task", task, "-api", "grpc"}},
		{"a sampling profile the model does not declare", []string{
			"-task", task, "-thinking", "on", "-sampling-profile", "creative"}},
		{"a model profile that is not there", []string{
			"-task", task, "-model-profile", repo("config", "profiles", "nope.json")}},
		{"a machine file that is not there", []string{
			"-task", task, "-machine", repo("config", "nope.json")}},
	} {
		if err := runEval(t, tc.args...); err == nil {
			t.Errorf("%s: expected a refusal", tc.name)
		}
	}
}

// Empty is a third state and must stay distinguishable from off: it leaves the model's own
// template default alone, which is not the same as setting the toggle false.
func TestParseThinkingKeepsUnsetDistinctFromOff(t *testing.T) {
	on, err := parseThinking("on")
	if err != nil || on == nil || !*on {
		t.Errorf(`parseThinking("on") = %v, %v`, on, err)
	}
	off, err := parseThinking("off")
	if err != nil || off == nil || *off {
		t.Errorf(`parseThinking("off") = %v, %v`, off, err)
	}
	unset, err := parseThinking("")
	if err != nil || unset != nil {
		t.Errorf(`parseThinking("") = %v, %v — empty must leave the template default alone`, unset, err)
	}
	if _, err := parseThinking("yes"); err == nil {
		t.Error("a value that is not on, off or empty must be refused rather than guessed")
	}
}

// Every row names the machine it was measured on, and it reads the name off the machine
// file rather than off the host: the file is what declares an envelope, and two envelopes
// are never compared. `-force` is set so the row is written whatever state this machine
// happens to be in — what is under test is the name on the row, not the preflight.
func TestEveryRowNamesTheMachineTheFileDeclares(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/props") {
			_, _ = w.Write([]byte(`{"model_path":"/m/Qwen.gguf","default_generation_settings":{"n_ctx":32768}}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"no tool call"}}]}`))
	}))
	defer srv.Close()

	machine := repo("config", "machine.json")
	results := filepath.Join(t.TempDir(), "rows.jsonl")
	// The task fails against this stub, which is a scored outcome and still writes a row.
	_ = runEval(t, "-task", repo("tasks", "toolcall-read-file.json"), "-endpoint", srv.URL,
		"-machine", machine, "-model-profile", repo("config", "profiles", "qwen3.8.json"),
		"-results", results, "-force")

	b, err := os.ReadFile(results)
	if err != nil {
		t.Fatalf("no row was written: %v", err)
	}
	var row eval.Row
	if err := json.Unmarshal([]byte(strings.SplitN(strings.TrimSpace(string(b)), "\n", 2)[0]), &row); err != nil {
		t.Fatalf("row: %v", err)
	}
	m, err := eval.LoadMachine(machine)
	if err != nil {
		t.Fatal(err)
	}
	if row.Machine != m.Name {
		t.Errorf("row names machine %q, want %q from %s", row.Machine, m.Name, machine)
	}
}
