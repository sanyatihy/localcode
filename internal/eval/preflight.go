package eval

import "fmt"

// Preflight is the memory state read before the first task and the verdict taken from it.
type Preflight struct {
	HeadroomGB    float64 `json:"headroom_gb"`
	MinHeadroomGB float64 `json:"min_headroom_gb"`
	FreeGB        float64 `json:"free_gb"`
	SwapUsedMB    float64 `json:"swap_used_mb"`
	Carries       bool    `json:"carries"`
	OK            bool    `json:"ok"`
}

// SamplePreflight is a package variable for the same reason Now is: it is the only
// nondeterminism here, and a test sets it directly rather than needing a machine.
var SamplePreflight = func() MemSample { return sampleMemory() }

// CheckPreflight decides from a reading already taken, against a threshold passed in.
//
// The verdict is headroom — total memory less wired and anonymous — and not free memory,
// which mem.go documents as no pressure signal at all: across 0010's 60-run sweep it read
// 0.16–0.65 GB whether the run swapped or not, so any floor on it refuses everything.
func CheckPreflight(s MemSample, minHeadroomGB float64) Preflight {
	return Preflight{
		HeadroomGB:    s.HeadroomGB(),
		MinHeadroomGB: minHeadroomGB,
		FreeGB:        s.FreeGB,
		SwapUsedMB:    s.SwapUsedMB,
		Carries:       s.OK && s.TotalGB > 0 && s.HeadroomGB() >= minHeadroomGB,
		OK:            s.OK && s.TotalGB > 0,
	}
}

// Refusal is what a command prints instead of starting.
func (p Preflight) Refusal() string {
	if !p.OK {
		return "the platform did not answer the memory probe, so a sweep cannot be checked; start it anyway with -force"
	}
	return fmt.Sprintf("the machine cannot carry this sweep: %.2f GB headroom against a %.2f GB floor "+
		"(%.2f GB free, %.0f MB of swap already in use); start it anyway with -force",
		p.HeadroomGB, p.MinHeadroomGB, p.FreeGB, p.SwapUsedMB)
}
