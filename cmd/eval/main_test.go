package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
