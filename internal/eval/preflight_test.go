package eval

import (
	"strings"
	"testing"
)

// A machine loaded like 0014's measured one: 20.89 GB wired serving at 32k, 6.61 GB of
// apps, 32 GB total — 4.5 GB of headroom, and free memory near zero either way.
func loaded(headroomGB float64) MemSample {
	return MemSample{
		TotalGB: 32, WiredGB: 20.89, AnonymousGB: 32 - 20.89 - headroomGB,
		FreeGB: 0.31, SwapUsedMB: 1631, OK: true,
	}
}

func TestCheckPreflight(t *testing.T) {
	tests := []struct {
		name    string
		sample  MemSample
		floor   float64
		carries bool
	}{
		{"headroom above the floor carries", loaded(6.0), 4.0, true},
		{"exactly at the floor carries", loaded(4.0), 4.0, true},
		{"below the floor is refused", loaded(1.5), 4.0, false},
		// The reading that broke the first version: free memory sat at 0.31 GB through
		// 0010's whole sweep, clean runs included.
		{"near-zero free memory does not decide it", loaded(6.0), 4.0, true},
		{"a silent platform refuses rather than guesses", MemSample{}, 4.0, false},
		{"no total means no verdict", MemSample{WiredGB: 4, AnonymousGB: 4, OK: true}, 4.0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := CheckPreflight(tc.sample, tc.floor)
			if p.Carries != tc.carries {
				t.Errorf("carries = %v, want %v (headroom %.2f GB, floor %.2f)",
					p.Carries, tc.carries, p.HeadroomGB, p.MinHeadroomGB)
			}
			if p.FreeGB != tc.sample.FreeGB || p.SwapUsedMB != tc.sample.SwapUsedMB {
				t.Errorf("verdict does not carry the numbers it read: %+v", p)
			}
		})
	}
}

func TestPreflightRefusalNamesItsNumbers(t *testing.T) {
	got := CheckPreflight(loaded(1.5), 4.0).Refusal()
	for _, want := range []string{"1.50 GB headroom", "4.00 GB floor", "0.31 GB free", "1631 MB", "-force"} {
		if !strings.Contains(got, want) {
			t.Errorf("refusal %q does not name %q", got, want)
		}
	}
	if unmeasured := (Preflight{}).Refusal(); !strings.Contains(unmeasured, "did not answer") {
		t.Errorf("an unmeasured machine refuses with %q", unmeasured)
	}
}

// The sampler must produce a verdict-shaped reading on the machine it runs on, or the
// check is testable and useless. Skipped where the platform does not answer, which is CI.
func TestSampleMemoryReadsWhatTheVerdictNeeds(t *testing.T) {
	s := SamplePreflight()
	if !s.OK {
		t.Skip("platform did not answer the memory probe")
	}
	if s.TotalGB <= 0 || s.WiredGB <= 0 || s.AnonymousGB <= 0 {
		t.Fatalf("sampler returned nothing to decide on: %+v", s)
	}
	if h := s.HeadroomGB(); h < 0 || h > s.TotalGB {
		t.Errorf("headroom %.2f GB is impossible against %.2f GB total", h, s.TotalGB)
	}
}
