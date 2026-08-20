package eval

import (
	"os"
	"path/filepath"
	"strings"
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

// The committed floor is what every sweep is judged against, so it has to load and to be
// a number this machine can actually clear: 0014 measured 27.50 GB resident with the model
// serving at 32k and an editor open, which leaves 4.5.
func TestTheCommittedMachineFileLoads(t *testing.T) {
	m, err := LoadMachine("../../config/machine.json")
	if err != nil {
		t.Fatalf("committed machine config: %v", err)
	}
	if m.MinHeadroomGB <= 0 || m.MinHeadroomGB > 8 {
		t.Errorf("min_headroom_gb = %.2f, which is outside anything 0014 measured", m.MinHeadroomGB)
	}
}

// A file that names no floor must not read as a machine with room.
func TestLoadMachineRefusesAFloorlessFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "machine.json")
	if err := os.WriteFile(p, []byte(`{"note":"nothing here"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMachine(p); err == nil {
		t.Error("a config with no floor was accepted")
	}
	if _, err := LoadMachine(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Error("a missing config was accepted")
	}
}

// Both sides of the decision a command makes, on readings this process supplies rather
// than on the machine it happens to run on: a sweep starts, or it is refused and says why.
// Neither side needs a server, and neither needs the machine to be in any particular state.
func TestRefuseCoversBothSides(t *testing.T) {
	// 0014's measured machine: 20.89 GB wired serving 32k, 6.61 GB of apps, 32 GB total.
	carries := MemSample{TotalGB: 32, WiredGB: 20.89, AnonymousGB: 6.61, FreeGB: 0.31, SwapUsedMB: 1631, OK: true}
	if why := Check(carries, 4.0).Refuse(); why != "" {
		t.Errorf("a machine with 4.5 GB headroom was refused: %s", why)
	}

	// The same machine with a browser open, which 0014 says does not fit beside the model.
	cannot := MemSample{TotalGB: 32, WiredGB: 20.89, AnonymousGB: 9.5, FreeGB: 0.31, SwapUsedMB: 4096, OK: true}
	why := Check(cannot, 4.0).Refuse()
	if why == "" {
		t.Fatal("a machine with 1.6 GB headroom was allowed to start")
	}
	for _, want := range []string{"1.61 GB headroom", "4.00 GB floor", "0.31 GB free", "4096 MB", "-force"} {
		if !strings.Contains(why, want) {
			t.Errorf("refusal %q does not name %q", why, want)
		}
	}

	// A platform that did not answer is refused rather than assumed roomy.
	if why := Check(MemSample{}, 4.0).Refuse(); !strings.Contains(why, "did not answer") {
		t.Errorf("an unmeasured machine refused with %q", why)
	}
}

// Sample is the seam that keeps the two sides above off this machine. A command that read
// the machine directly could not be tested at all.
func TestSampleIsSubstitutable(t *testing.T) {
	original := Sample
	t.Cleanup(func() { Sample = original })
	Sample = func() MemSample { return MemSample{TotalGB: 32, WiredGB: 30, AnonymousGB: 1, OK: true} }
	if p := Check(Sample(), 4.0); p.Carries {
		t.Errorf("substituted reading was ignored: %+v", p)
	}
}
