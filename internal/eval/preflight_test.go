package eval

import (
	"strings"
	"testing"
)

// Both sides of the verdict, on fixed readings: a machine with headroom carries a sweep,
// one without is refused, and neither side needs a running server — the reading is
// injected through SamplePreflight rather than taken from this machine.
func TestCheckPreflight(t *testing.T) {
	const floor = 11.0 // 0014's arithmetic: 32 GB minus 20.89 wired at 32k minus 6.61 of apps
	tests := []struct {
		name    string
		sample  MemSample
		carries bool
	}{
		{"headroom carries", MemSample{FreeGB: 14.2, SwapUsedMB: 0, OK: true}, true},
		{"at the floor carries", MemSample{FreeGB: 11.0, SwapUsedMB: 512, OK: true}, true},
		{"no headroom is refused", MemSample{FreeGB: 3.7, SwapUsedMB: 4096, OK: true}, false},
		// Swap in use is reported with the verdict but does not decide it: free memory is
		// the pre-run signal (the model must fit on top of what is resident), and a swap
		// floor would be a second threshold this box did not ask for.
		{"free but already swapping still carries", MemSample{FreeGB: 12.5, SwapUsedMB: 8192, OK: true}, true},
		{"a silent platform refuses rather than guesses", MemSample{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := CheckPreflight(tc.sample, floor)
			if p.Carries != tc.carries {
				t.Errorf("carries = %v, want %v (free %.2f GB, swap %.0f MB)",
					p.Carries, tc.carries, p.FreeGB, p.SwapUsedMB)
			}
			if p.FreeGB != tc.sample.FreeGB || p.SwapUsedMB != tc.sample.SwapUsedMB || p.MinFreeGB != floor {
				t.Errorf("verdict does not carry the numbers it read: %+v", p)
			}
		})
	}
}

func TestPreflightRefusalNamesItsNumbers(t *testing.T) {
	p := CheckPreflight(MemSample{FreeGB: 3.7, SwapUsedMB: 4096, OK: true}, 11.0)
	got := p.Refusal()
	for _, want := range []string{"3.70 GB free", "4096 MB of swap in use", "11.00 GB floor", "-force"} {
		if !strings.Contains(got, want) {
			t.Errorf("refusal %q does not name %q", got, want)
		}
	}
}
