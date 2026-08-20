package eval

import (
	"testing"
)

// The preflight is arithmetic on the sample, so both sides are testable without a machine
// in either state: headroom above the floor carries, below it refuses.
func TestPreflightCarriesAndRefuses(t *testing.T) {
	carry := MemSample{TotalGB: 32, WiredGB: 20.89, AnonymousGB: 6.61, OK: true}
	if p := Check(carry, 4); !p.Carries || p.HeadroomGB < 4.49 || p.HeadroomGB > 4.6 {
		t.Errorf("32 GB machine with ~4.5 GB headroom refused or misread: %+v", p)
	}
	refuse := MemSample{TotalGB: 32, WiredGB: 28, AnonymousGB: 6, OK: true}
	if p := Check(refuse, 4); p.Carries {
		t.Errorf("machine with 0 GB headroom carried a 4 GB floor: %+v", p)
	}
	// A sample the platform did not answer is refused: no number is not a big number.
	if p := Check(MemSample{}, 0); p.Carries {
		t.Errorf("unanswered sample carried: %+v", p)
	}
}

// The sampler must read wired and anonymous off vm_stat's labels, and total off hw.memsize.
func TestSampleReadsWiredAndAnonymous(t *testing.T) {
	s := sampleMemory()
	if !s.OK {
		t.Skip("platform did not answer")
	}
	if s.TotalGB <= 0 || s.WiredGB <= 0 || s.AnonymousGB <= 0 {
		t.Errorf("missing a headroom input: %+v", s)
	}
	if h := s.Headroom(); h < 0 || h > s.TotalGB {
		t.Errorf("headroom outside the machine: %v of %v GB", h, s.TotalGB)
	}
}
