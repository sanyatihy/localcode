package eval

import "fmt"

// Preflight is the memory state read once, before the first task, and the decision made
// from it: can this machine carry a sweep, or will its timings measure the pager? 0010's
// two-hour sweep at 65,536 produced exactly such timings — 25 of 60 rows grew swap — and
// nothing said so until the machine time was spent. A run that swaps measures paging
// rather than inference (0003), so the question is asked before the first task instead of
// answered by the rows afterwards.
type Preflight struct {
	FreeGB     float64 `json:"free_gb"`
	SwapUsedMB float64 `json:"swap_used_mb"`
	MinFreeGB  float64 `json:"min_free_gb"` // the threshold this verdict was made against
	Carries    bool    `json:"carries"`
	OK         bool    `json:"ok"` // false when the platform did not answer; a refusal on silence would refuse every machine
}

// SamplePreflight reads the memory state once. It is a package variable rather than a
// plain function for the same reason Now is: it is the only nondeterminism in this
// check, and a test that wants a fixed reading sets it directly — neither side of the
// verdict then needs a machine, let alone a running server.
var SamplePreflight = func() MemSample { return sampleMemory() }

// CheckPreflight decides from a reading already taken, so the decision is testable on
// fixed numbers and the reading stays one call site. The threshold is passed in rather
// than held here: what counts as too little headroom is a property of the machine, it
// lives in config/ with the other machine properties, and a constant in Go would be
// re-derived by anyone on other hardware.
func CheckPreflight(s MemSample, minFreeGB float64) Preflight {
	p := Preflight{FreeGB: s.FreeGB, SwapUsedMB: s.SwapUsedMB, MinFreeGB: minFreeGB, OK: s.OK}
	// Free memory is the pre-run signal even though it is not a pressure signal during a
	// run: what must fit is the model's wired footprint on top of what is already
	// resident, and 0014 measured that arithmetic — 20.89 GB wired at 32k plus 6.61 GB of
	// apps against 32 GB leaves roughly 11 GB for everything else. Swap in use is carried
	// on the verdict so a refusal fails loudly with both numbers it read, not one.
	p.Carries = s.OK && s.FreeGB >= minFreeGB
	return p
}

// Refusal is the message a command prints when it refuses to start: the numbers that
// were read, the threshold they failed against, and what would override the refusal.
func (p Preflight) Refusal() string {
	if p.OK {
		return fmt.Sprintf("the machine cannot carry this sweep: %.2f GB free with %.0f MB of swap in use, "+
			"below the %.2f GB floor; start it anyway with -force", p.FreeGB, p.SwapUsedMB, p.MinFreeGB)
	}
	return "the platform did not answer the memory probe; a sweep cannot be checked and will not start unforced"
}
