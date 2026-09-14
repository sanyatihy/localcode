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
	carry := MemSample{Platform: platformDarwin, TotalGB: 32, WiredGB: 20.89, AnonymousGB: 6.61, OK: true}
	if p := Check(carry, 4); !p.Carries || p.HeadroomGB < 4.49 || p.HeadroomGB > 4.6 {
		t.Errorf("32 GB machine with ~4.5 GB headroom refused or misread: %+v", p)
	}
	refuse := MemSample{Platform: platformDarwin, TotalGB: 32, WiredGB: 28, AnonymousGB: 6, OK: true}
	if p := Check(refuse, 4); p.Carries {
		t.Errorf("machine with 0 GB headroom carried a 4 GB floor: %+v", p)
	}
	// A sample the platform did not answer is refused: no number is not a big number.
	if p := Check(MemSample{}, 0); p.Carries {
		t.Errorf("unanswered sample carried: %+v", p)
	}
}

// This machine, read by whichever sampler is built for it, and held to that platform's own
// rule rather than to the other's. Where nothing answers the sample is not OK and carries no
// numbers at all: a plausible zero on a row reads as a machine with nothing wired, which is
// a reading a preflight would carry rather than refuse.
func TestSampleReadsWhatItsOwnRuleNeeds(t *testing.T) {
	s := sampleMemory()
	if !s.OK {
		if s.TotalGB != 0 || s.WiredGB != 0 || s.AnonymousGB != 0 || s.FreeGB != 0 || s.SwapUsedMB != 0 {
			t.Fatalf("a platform that did not answer reported numbers anyway: %+v", s)
		}
		return
	}
	switch s.Platform {
	case platformDarwin:
		if s.TotalGB <= 0 || s.WiredGB <= 0 || s.AnonymousGB <= 0 {
			t.Errorf("missing a headroom input: %+v", s)
		}
	case platformLinux:
		if s.TotalGB <= 0 || s.AvailableGB <= 0 {
			t.Errorf("missing a headroom input: %+v", s)
		}
		// Linux has no counter corresponding to either of these, and its rule wants
		// neither. They stay at zero, and a zero here is read as "no such reading"
		// only because the platform on the sample says which rule applies.
		if s.WiredGB != 0 || s.AnonymousGB != 0 {
			t.Errorf("a Linux sample carries a macOS counter: %+v", s)
		}
	default:
		t.Fatalf("an answered sample names no platform, so it has no headroom rule: %+v", s)
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
	// The name every row taken here carries. It has to be the same token the laptop's data
	// files are named with, or a file's rows and the file itself would name two machines.
	fromFile, ok := MachineFromFileName("2026-08-17-m2max-32gb-tier1-matrix.jsonl")
	if !ok || m.Name != fromFile {
		t.Errorf("the laptop's machine file calls it %q; its data files say %q", m.Name, fromFile)
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
	// A file that names no machine would write rows nobody can attribute to an envelope,
	// and they would read as the laptop's.
	nameless := filepath.Join(t.TempDir(), "machine.json")
	if err := os.WriteFile(nameless, []byte(
		`{"min_headroom_gb":4.0,"desk_profiles":[{"name":"attended","ceiling_tokens":57344}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMachine(nameless); err == nil {
		t.Error("a machine file that names no machine was accepted")
	}
}

// Both sides of the decision a command makes, on readings this process supplies rather
// than on the machine it happens to run on: a sweep starts, or it is refused and says why.
// Neither side needs a server, and neither needs the machine to be in any particular state.
func TestRefuseCoversBothSides(t *testing.T) {
	// 0014's measured machine: 20.89 GB wired serving 32k, 6.61 GB of apps, 32 GB total.
	carries := MemSample{Platform: platformDarwin, TotalGB: 32, WiredGB: 20.89, AnonymousGB: 6.61,
		FreeGB: 0.31, SwapUsedMB: 1631, OK: true}
	if why := Check(carries, 4.0).Refuse(); why != "" {
		t.Errorf("a machine with 4.5 GB headroom was refused: %s", why)
	}

	// The same machine with a browser open, which 0014 says does not fit beside the model.
	cannot := MemSample{Platform: platformDarwin, TotalGB: 32, WiredGB: 20.89, AnonymousGB: 9.5,
		FreeGB: 0.31, SwapUsedMB: 4096, OK: true}
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
	Sample = func() MemSample {
		return MemSample{Platform: platformDarwin, TotalGB: 32, WiredGB: 30, AnonymousGB: 1, OK: true}
	}
	if p := Check(Sample(), 4.0); p.Carries {
		t.Errorf("substituted reading was ignored: %+v", p)
	}
}

// One vm_stat, as a 32 GB M2 Max prints it: the labels this parser is keyed off sit in
// different columns on different macOS versions, and the page size is 16384 here — it was
// assumed to be 4096 once, and every figure taken then was four times too small.
const vmStatFixture = `Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                               20316.
Pages active:                             655360.
Pages inactive:                           131072.
Pages speculative:                        12345.
Pages throttled:                          0.
Pages wired down:                         1369047.
Pages purgeable:                          8192.
"Translation faults":                     1234567890.
Pages copy-on-write:                      12345678.
Pages zero filled:                        987654321.
Pages reactivated:                        123456.
Pages purged:                             654321.
File-backed pages:                        130432.
Anonymous pages:                          433193.
Pages stored in compressor:               262144.
Pages occupied by compressor:             65536.
`

// One /proc/meminfo, as an Arm Ubuntu box with 128 GB prints it. Kept whole rather than cut
// to the four fields read: the parser's job is to find them among the forty that are not,
// and a fixture of only what it wants cannot show that it did.
const procMeminfoFixture = `MemTotal:       131530240 kB
MemFree:        118226944 kB
MemAvailable:   127336448 kB
Buffers:          210944 kB
Cached:          9216000 kB
SwapCached:            0 kB
Active:          4194304 kB
Inactive:        6291456 kB
Active(anon):    1048576 kB
Inactive(anon):   131072 kB
Active(file):    3145728 kB
Inactive(file):  6160384 kB
Unevictable:       16384 kB
Mlocked:           16384 kB
SwapTotal:       8388608 kB
SwapFree:        8126464 kB
Dirty:              1024 kB
Writeback:             0 kB
AnonPages:       1179648 kB
Mapped:           524288 kB
Shmem:             65536 kB
KReclaimable:     327680 kB
Slab:             655360 kB
SReclaimable:     327680 kB
SUnreclaim:       327680 kB
KernelStack:       32768 kB
PageTables:        49152 kB
Bounce:                0 kB
WritebackTmp:          0 kB
CommitLimit:    74153728 kB
Committed_AS:    3145728 kB
VmallocTotal:   133009506240 kB
VmallocUsed:      131072 kB
VmallocChunk:          0 kB
Percpu:            65536 kB
HugePages_Total:       0
HugePages_Free:        0
Hugepagesize:       2048 kB
`

func closeTo(got, want float64) bool { return got-want < 0.01 && want-got < 0.01 }

// The macOS parser, on the state docs/TECH.md's envelope is stated in: 20.89 GB wired
// serving at 32k, 6.61 GB of apps, free pinned near zero.
func TestVMStatParserReadsTheLabels(t *testing.T) {
	s := parseVMStat(vmStatFixture, 16384)
	if !closeTo(s.WiredGB, 20.89) || !closeTo(s.AnonymousGB, 6.61) || !closeTo(s.FreeGB, 0.31) {
		t.Fatalf("got %+v, want 20.89 wired, 6.61 anonymous, 0.31 free", s)
	}
	// Three numbers of the same shape sit on the swapusage line, and the one the run
	// depended on is the one labelled used.
	if got, ok := parseSwapUsage("total = 4096.00M  used = 1465.44M  free = 2630.56M  (encrypted)"); got != 1465.44 || !ok {
		t.Fatalf("swap used = %v (read %v), want 1465.44 — the used figure, not the total beside it", got, ok)
	}
	// A machine that has never swapped reports 0.00M, so an unread line cannot come back as
	// a zero: the two mean opposite things and one of them voids a run.
	if got, ok := parseSwapUsage("vm.swapusage: unknown oid"); ok {
		t.Fatalf("an unreadable swap line was read as %v MB in use", got)
	}
}

// The Linux parser reads the fields its own headroom rule needs, and swap as total less
// free, which is what macOS reports directly.
func TestMeminfoParserReadsWhatTheLinuxRuleNeeds(t *testing.T) {
	s := parseMeminfo(procMeminfoFixture)
	if !s.OK || s.Platform != platformLinux {
		t.Fatalf("got %+v, want an OK Linux sample", s)
	}
	if !closeTo(s.TotalGB, 125.4375) || !closeTo(s.AvailableGB, 121.4375) || !closeTo(s.FreeGB, 112.75) {
		t.Fatalf("got %+v, want 125.4375 total, 121.4375 available, 112.75 free", s)
	}
	if !closeTo(s.SwapUsedMB, 256) {
		t.Fatalf("swap used = %v MB, want 256 — SwapTotal less SwapFree", s.SwapUsedMB)
	}
	// No wired counter exists here, and the Linux rule does not want one. A sample read
	// with the macOS arithmetic would report 125.44 GB of headroom on a full machine.
	if !closeTo(s.Headroom(), s.AvailableGB) {
		t.Fatalf("headroom = %v, want MemAvailable %v", s.Headroom(), s.AvailableGB)
	}
}

// A kernel too old to report MemAvailable leaves the Linux rule with nothing to refuse a
// sweep with, and a file with no swap line leaves the contamination check reporting a delta
// of zero it never read. Neither is a reading, and the preflight says so.
func TestAMeminfoMissingAFieldItNeedsIsRefused(t *testing.T) {
	for _, missing := range []string{"MemAvailable:", "MemTotal:", "SwapTotal:", "SwapFree:"} {
		var without []string
		for _, line := range strings.Split(procMeminfoFixture, "\n") {
			if !strings.HasPrefix(line, missing) {
				without = append(without, line)
			}
		}
		s := parseMeminfo(strings.Join(without, "\n"))
		if s.OK || s.TotalGB != 0 || s.SwapUsedMB != 0 {
			t.Fatalf("without %s: got %+v, want an unanswered sample rather than a partial one", missing, s)
		}
		if p := Check(s, 4); p.Carries {
			t.Fatalf("without %s: a partial sample carried a sweep: %+v", missing, p)
		}
		if why := Check(s, 4).Refuse(); !strings.Contains(why, "did not answer") {
			t.Errorf("without %s: refusal %q does not say the machine was not read", missing, why)
		}
	}
}

// Each platform's numbers under the other's rule are wrong by tens of gigabytes, so the
// rule follows the sample rather than the machine the report is read on.
func TestHeadroomFollowsThePlatformTheSampleWasTakenOn(t *testing.T) {
	mac := parseVMStat(vmStatFixture, 16384)
	mac.Platform, mac.TotalGB, mac.OK = platformDarwin, 32, true
	if !closeTo(mac.Headroom(), 32-20.89-6.61) {
		t.Errorf("macOS headroom = %v, want total less wired and anonymous", mac.Headroom())
	}
	linux := parseMeminfo(procMeminfoFixture)
	if !closeTo(linux.Headroom(), 121.4375) {
		t.Errorf("Linux headroom = %v, want MemAvailable", linux.Headroom())
	}
	// A sample that names no platform has no rule, and reporting one machine's arithmetic
	// for the other is how a sweep starts on a box that cannot carry it.
	unnamed := MemSample{TotalGB: 32, WiredGB: 20.89, AnonymousGB: 6.61, OK: true}
	if unnamed.Headroom() != 0 || Check(unnamed, 4).Carries {
		t.Errorf("a sample naming no platform was given one: %+v", Check(unnamed, 4))
	}
}
