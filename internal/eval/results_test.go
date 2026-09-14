package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sanyatihy/localcode/internal/build"
)

// Tool-call validity is a derived metric, and the derivation is the whole point:
// a parseable call to the wrong tool is a reasoning failure, an unparseable one is
// a format failure, and a server error is neither. Conflating them would send 0005
// after the wrong lever.
func TestToolCallValid(t *testing.T) {
	tests := []struct {
		kind           string
		outcome        Outcome
		valid, applies bool
	}{
		{"toolcall", Pass, true, true},
		{"toolcall", FailWrongTool, true, true}, // valid JSON, wrong choice
		{"toolcall", FailArgs, true, true},      // valid JSON, wrong contents
		{"toolcall", FailBadJSON, false, true},  // the format failure
		{"toolcall", FailNoCall, false, true},   // prose instead of a call
		{"toolcall", FailServer, false, false},  // not the model's doing
		{"patch", Pass, false, false},           // metric does not apply
		{"retrieval", FailRetrieval, false, false},
	}
	for _, tc := range tests {
		r := Row{Kind: tc.kind, Outcome: tc.outcome}
		valid, applies := r.ToolCallValid()
		if valid != tc.valid || applies != tc.applies {
			t.Errorf("%s/%s: got (valid=%v, applicable=%v), want (%v, %v)",
				tc.kind, tc.outcome, valid, applies, tc.valid, tc.applies)
		}
	}
}

// A row is evidence only if somebody can get back to the code that wrote it, and the
// driver's own arithmetic has moved under rows before — 0043 changed what every session is
// budgeted, and rows either side were otherwise indistinguishable.
func TestEveryRowNamesTheBuildThatWroteIt(t *testing.T) {
	r := NewRow("cfg", 0, "off", "", Sampling{}, ServerProps{}, "toolcall",
		Result{TaskID: "t", Outcome: Pass})
	if r.Driver == "" {
		t.Fatal("a row with no driver cannot be attributed to a build")
	}
	// Under `go test` the binary carries no VCS stamp, so what must never happen is a blank
	// that reads as a revision.
	if r.Driver != build.Revision() {
		t.Fatalf("driver = %q, want the build's own answer %q", r.Driver, build.Revision())
	}
}

// A row that names no machine takes it from the file's name, which is the only record of
// which machine the rows written before the field came off — and they came off two.
func TestTheMachineIsReadOffAResultsFileName(t *testing.T) {
	for name, want := range map[string]string{
		"2026-08-17-m2max-32gb-ceiling-ladder.jsonl":   "m2max-32gb",
		"2026-09-14-m5max-36gb-tier1.jsonl":            "m5max-36gb",
		"docs/data/2026-09-14-m5max-36gb-screen.jsonl": "m5max-36gb",
		"2026-09-20-gb10-128gb-ladder.jsonl":           "gb10-128gb",
	} {
		got, ok := MachineFromFileName(name)
		if !ok || got != want {
			t.Errorf("%s names machine %q (read %v), want %q", name, got, ok, want)
		}
	}
	// A file nobody named after a machine says nothing about one, and a reader must refuse
	// rather than assume: results/tier1.jsonl is written on whichever machine ran it.
	for _, name := range []string{"results/tier1.jsonl", "rows.jsonl", "2026-09-14-tier1.jsonl",
		"m5max-36gb-tier1.jsonl"} {
		if got, ok := MachineFromFileName(name); ok {
			t.Errorf("%s was read as machine %q", name, got)
		}
	}
}

// Every committed data file has to be readable by that rule, since none of their rows names
// a machine: a file the rule cannot read is evidence the report would refuse to summarise.
func TestEveryCommittedDataFileNamesItsMachine(t *testing.T) {
	entries, err := os.ReadDir("../../docs/data")
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		seen++
		if _, ok := MachineFromFileName(e.Name()); !ok {
			t.Errorf("%s names no machine, so its rows belong to no envelope", e.Name())
		}
	}
	if seen == 0 {
		t.Fatal("no data files were checked")
	}
}
