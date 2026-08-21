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
	if err := eval.AppendRow(path, eval.Row{Config: "a", Kind: "toolcall", Outcome: eval.Pass}); err != nil {
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
