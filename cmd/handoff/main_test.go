package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRefusesWhatItCannotDrive(t *testing.T) {
	if err := (&config{sessions: 5}).validate(); err == nil {
		t.Error("-doc names the work; without it there is no box to run")
	}
	doc := filepath.Join(t.TempDir(), "0001-x.md")
	if err := os.WriteFile(doc, []byte("## Tasks\n- [ ] a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A bound below one runs nothing, which is a typo rather than an instruction.
	if err := (&config{doc: doc, checkout: ".", sessions: 0}).validate(); err == nil {
		t.Error("-sessions 0 must be refused")
	}
}

// The prompt and every configured path are resolved against the checkout, because a feature
// is worked in a worktree of its own and a relative path would resolve against wherever the
// driver happened to be launched.
func TestValidateResolvesAgainstTheCheckout(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "0001-x.md")
	if err := os.WriteFile(doc, []byte("## Tasks\n- [ ] a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &config{doc: "0001-x.md", checkout: dir, sessions: 1,
		env: "harness/claude-code/claude-code.env", settings: "harness/claude-code/hooks.json",
		refusals: "results/precompact.jsonl"}
	if err := c.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	for name, got := range map[string]string{
		"doc": c.doc, "env": c.env, "settings": c.settings, "refusals": c.refusals,
		"checkout": c.checkout,
	} {
		if !filepath.IsAbs(got) {
			t.Errorf("%s stayed relative: %q", name, got)
		}
	}
	// Built from the doc, and relative to the checkout: an absolute path in the prompt
	// would name a directory the session cannot see.
	if !strings.Contains(c.prompt, "0001-x.md") || strings.Contains(c.prompt, dir) {
		t.Errorf("prompt should name the doc relative to the checkout, got %q", c.prompt)
	}
}

func TestReadBoxesRefusesADocWithNoTasks(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(empty, []byte("## Design\nno boxes here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readBoxes(empty); err == nil {
		t.Error("a doc with no ## Tasks boxes gives the driver nothing to do and must say so")
	}
	if _, err := readBoxes(filepath.Join(dir, "nope.md")); err == nil {
		t.Error("a doc that is not there must be refused")
	}
}

func TestStatusNamesTheThreeOutcomes(t *testing.T) {
	for _, tc := range []struct {
		r    row
		want string
	}{
		{row{Ticked: true}, "TICKED"},
		{row{Err: "boom"}, "ERROR"},
		{row{}, "ONWARD"},
		// A ticked box is ticked even if the session also errored: the doc is the record,
		// not what the session says about itself.
		{row{Ticked: true, Err: "boom"}, "TICKED"},
	} {
		if got := strings.TrimSpace(status(tc.r)); got != tc.want {
			t.Errorf("status(%+v) = %q, want %q", tc.r, got, tc.want)
		}
	}
}

func TestTailKeepsTheEnd(t *testing.T) {
	if got := tail("abcdef", 3); got != "…def" {
		t.Errorf("tail = %q, want the last three characters", got)
	}
	if got := tail("  ab  ", 10); got != "ab" {
		t.Errorf("tail = %q, want it trimmed and whole", got)
	}
}
