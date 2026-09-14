package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sanyatihy/localcode/internal/eval"
)

func decodeRow(sec float64, tokens int) eval.Row {
	return eval.Row{DecodeMeasured: true, DecodeSeconds: sec, CompletionTokens: tokens, MemMeasured: true}
}

// A run the machine capped measured the cap, and one that swapped measured the pager.
// Both are void on the same footing: counted, named, and kept out of the mean.
func TestPairedRatioVoidsThrottledAndSwappedRuns(t *testing.T) {
	p := &pairAgg{}
	p.add(decodeRow(1, 10))
	throttled := decodeRow(4, 10)
	throttled.Throttled, throttled.SpeedLimit = true, 60
	p.add(throttled)
	swapped := decodeRow(4, 10)
	swapped.SwapDeltaMB = 128
	p.add(swapped)

	if got := len(p.secPerToken); got != 1 {
		t.Fatalf("kept %d samples, want 1 — a void run reached the mean", got)
	}
	if p.throttled != 1 || p.swapped != 1 {
		t.Fatalf("throttled=%d swapped=%d, want 1 and 1", p.throttled, p.swapped)
	}
}

// One report, one reading of "void". The summary allowed 20 MB of slack because macOS
// moves swap around without the run causing it; the paired section voided any growth at
// all, so the same run read clean in one half and void in the other.
func TestBothHalvesOfTheReportVoidASwappedRunAtOneThreshold(t *testing.T) {
	// Inside the slack: neither half may call this void.
	slack := decodeRow(1, 10)
	slack.SwapDeltaMB = swapSlackMB
	p := &pairAgg{}
	p.add(slack)
	if p.swapped != 0 || len(p.secPerToken) != 1 {
		t.Fatalf("swap within the slack was voided: swapped=%d kept=%d",
			p.swapped, len(p.secPerToken))
	}

	// Past it: both must.
	past := decodeRow(1, 10)
	past.SwapDeltaMB = swapSlackMB + 1
	p = &pairAgg{}
	p.add(past)
	if p.swapped != 1 || len(p.secPerToken) != 0 {
		t.Fatalf("swap past the slack reached the mean: swapped=%d kept=%d",
			p.swapped, len(p.secPerToken))
	}

	dir := t.TempDir()
	results := filepath.Join(dir, "rows.jsonl")
	for _, r := range []eval.Row{slack, past} {
		r.Config, r.TaskID, r.Kind, r.Outcome = "c", "t", "toolcall", eval.Pass
		r.Machine = "m2max-32gb" // a row written today names its machine; this is not that test
		if err := eval.AppendRow(results, r); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runReport(t, "-results", results)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "VOID: 1 run(s) swapped") {
		t.Fatalf("the summary must void exactly the run the paired section does:\n%s", out)
	}
}

// A stall is not a slow decode. One sample far past the median of its own side is
// rejected, and rejecting it is recorded rather than silent.
func TestPairedRatioRejectsAStall(t *testing.T) {
	p := &pairAgg{}
	for range 4 {
		p.add(decodeRow(1, 10)) // 0.1 s/token
	}
	p.add(decodeRow(9, 10)) // 0.9 s/token, nine times the median

	kept, stalled := p.accepted()
	if len(kept) != 4 {
		t.Fatalf("kept %d samples, want 4", len(kept))
	}
	if stalled != 1 {
		t.Fatalf("stalled = %d, want 1", stalled)
	}
}

// The stall count is a property of the samples, not of how often they were read. The
// reporter reads the baseline's set once per candidate, and each read used to add the
// same stalls again — printing one void run as three.
func TestPairedStallCountSurvivesRepeatedReads(t *testing.T) {
	base, cand := &pairAgg{}, &pairAgg{}
	for range minPairs + 1 {
		base.add(decodeRow(2, 10))
		cand.add(decodeRow(1, 10))
	}
	base.add(decodeRow(40, 10)) // one stall, on the side every candidate is divided by

	var buf bytes.Buffer
	reportPaired(&buf, map[string]map[string]*pairAgg{"s": {"tuned": base, "candidate": cand}}, "tuned")
	if got := buf.String(); !strings.Contains(got, "(1 void: stalled") {
		t.Fatalf("one stall must be reported once, got:\n%s", got)
	}
}

// Lossless is the premise of the comparison, so a greedy mismatch takes the number away
// rather than appearing as a footnote beside it.
func TestPairedRatioVoidsOnFidelityMismatch(t *testing.T) {
	build := func(baseHash, candHash string) string {
		base, cand := &pairAgg{fidelity: baseHash}, &pairAgg{fidelity: candHash}
		for range minPairs {
			base.add(decodeRow(2, 10))
			cand.add(decodeRow(1, 10))
		}
		var buf bytes.Buffer
		reportPaired(&buf, map[string]map[string]*pairAgg{
			"s": {"tuned": base, "candidate": cand},
		}, "tuned")
		return buf.String()
	}

	same := build("abc", "abc")
	if !strings.Contains(same, "2.00x") {
		t.Fatalf("matching fidelity should report the ratio, got:\n%s", same)
	}
	differs := build("abc", "xyz")
	if !strings.Contains(differs, "VOID") || strings.Contains(differs, "2.00x") {
		t.Fatalf("a mismatch must void the ratio, got:\n%s", differs)
	}
	unchecked := build("", "")
	if !strings.Contains(unchecked, "fidelity unchecked") {
		t.Fatalf("an unrun check must say so rather than read as a pass, got:\n%s", unchecked)
	}
}

// Three is the floor because fewer is a coincidence rather than a measurement.
func TestPairedRatioRefusesTooFewPairs(t *testing.T) {
	base, cand := &pairAgg{}, &pairAgg{}
	base.add(decodeRow(2, 10))
	cand.add(decodeRow(1, 10))
	var buf bytes.Buffer
	reportPaired(&buf, map[string]map[string]*pairAgg{"s": {"tuned": base, "candidate": cand}}, "tuned")
	if !strings.Contains(buf.String(), "too few accepted pairs") {
		t.Fatalf("one pair must not produce a ratio, got:\n%s", buf.String())
	}
}

// report is the only command whose exit code is not a suite verdict: 2 means it could not
// read what it was pointed at, and anything it can read it summarises.
func runReport(t *testing.T, args ...string) (string, error) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	runErr := run(args, f, f)
	_ = f.Close()
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return string(b), runErr
}

func TestReportRefusesAResultsFileItCannotRead(t *testing.T) {
	if _, err := runReport(t, "-results", filepath.Join(t.TempDir(), "nope.jsonl")); err == nil {
		t.Error("a results file that is not there must be refused, not summarised as empty")
	}
}

// A file with no matching rows is not an error — a sweep may legitimately have written none
// under the label being asked about — and saying so beats printing an empty table.
func TestReportSaysWhenNothingMatched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.jsonl")
	if err := eval.AppendRow(path, eval.Row{Config: "a", Kind: "toolcall", Outcome: eval.Pass}); err != nil {
		t.Fatal(err)
	}
	out, err := runReport(t, "-results", path, "-config", "b")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out, "no rows matched") {
		t.Errorf("expected `no rows matched`, got:\n%s", out)
	}
}

// A row this build cannot parse is skipped with a note rather than ending the summary: a
// results file accumulates across features, and one bad line must not hide the rest.
func TestReportSurvivesAnUnparseableRow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.jsonl")
	if err := eval.AppendRow(path, eval.Row{Config: "a", Kind: "toolcall", Outcome: eval.Pass,
		Machine: "m2max-32gb"}); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{not json\n\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	out, err := runReport(t, "-results", path)
	if err != nil {
		t.Fatalf("one bad line must not end the summary: %v", err)
	}
	if !strings.Contains(out, "1/1") {
		t.Errorf("the readable row is missing from:\n%s", out)
	}
}

// Two envelopes are not one measurement. The laptop and the GB10 differ in memory,
// bandwidth and slot count, so a pass rate or a decode mean over both is a number about
// neither, and nothing in the table would show that it had happened.
func TestReportRefusesRowsFromTwoMachines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.jsonl")
	for _, machine := range []string{"m2max-32gb", "gb10-128gb"} {
		row := eval.Row{Config: "tuned", Kind: "toolcall", Outcome: eval.Pass, Machine: machine}
		if err := eval.AppendRow(path, row); err != nil {
			t.Fatal(err)
		}
	}
	_, err := runReport(t, "-results", path)
	if err == nil {
		t.Fatal("rows from two machines were summarised as one")
	}
	for _, want := range []string{"m2max-32gb", "gb10-128gb", "envelopes"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal %q does not name %q", err, want)
		}
	}
}

// fieldless writes rows exactly as every committed data file carries them: no `machine` key
// at all, which is what predates the field. eval.Row cannot express that, since it writes
// the key whatever it holds.
func fieldless(t *testing.T, path string, n int) {
	t.Helper()
	line := `{"config":"tuned","kind":"toolcall","outcome":"pass","mem_measured":true}` + "\n"
	if err := os.WriteFile(path, []byte(strings.Repeat(line, n)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Rows written before the field name no machine, and they did not all come off one: the
// laptop's files and the node's sit in docs/data together. The file's name is the record,
// and docs/data/README is what fixes it.
func TestReportReadsTheMachineOffTheFileName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "2026-09-14-m5max-36gb-tier1.jsonl")
	fieldless(t, path, 2)
	out, err := runReport(t, "-results", path)
	if err != nil {
		t.Fatalf("a file naming its machine was refused: %v", err)
	}
	if !strings.Contains(out, "machine: m5max-36gb") || !strings.Contains(out, "2/2") {
		t.Errorf("the summary does not name the machine the file was taken on:\n%s", out)
	}

	// A row that names its own machine is that machine's, whatever the file is called; the
	// laptop's rows and the node's are not one average because they share a directory.
	mixed := filepath.Join(t.TempDir(), "2026-09-14-m5max-36gb-tier1.jsonl")
	fieldless(t, mixed, 1)
	row := eval.Row{Config: "tuned", Kind: "toolcall", Outcome: eval.Pass, Machine: "m2max-32gb"}
	if err := eval.AppendRow(mixed, row); err != nil {
		t.Fatal(err)
	}
	if _, err := runReport(t, "-results", mixed); err == nil {
		t.Error("a file whose name and whose rows name different machines was summarised as one")
	}
}

// A file nobody named after a machine, holding rows that name none either, is not the
// laptop's by default: results/tier1.jsonl is written on whichever machine ran the sweep.
func TestReportRefusesRowsNoFileNameCanAttribute(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tier1.jsonl")
	fieldless(t, path, 1)
	_, err := runReport(t, "-results", path)
	if err == nil {
		t.Fatal("rows belonging to no envelope were summarised")
	}
	for _, want := range []string{"tier1.jsonl", "<date>-<machine>-<what>"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal %q does not name %q", err, want)
		}
	}
}
