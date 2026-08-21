package main

import (
	"bytes"
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

	kept := p.accepted()
	if len(kept) != 4 {
		t.Fatalf("kept %d samples, want 4", len(kept))
	}
	if p.stalled != 1 {
		t.Fatalf("stalled = %d, want 1", p.stalled)
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
