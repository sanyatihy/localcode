package eval

import (
	"strconv"
	"strings"
)

// MemSample is the memory state around one run. The vision requires every run to record
// it so a contaminated result is detected rather than assumed clean: on 32 GB the model
// plus a desktop can exhaust memory before a context is full, and every timing taken in
// that state measures paging rather than inference.
//
// Free memory is recorded but is not a pressure signal on macOS — it sits near zero
// whether idle or paging. The swap delta across a run is the signal.
//
// The fields a platform cannot read stay zero and Platform says which platform it is, so
// nothing reads a Linux sample with macOS arithmetic: there is no wired counter there, and
// zero wired would read as a machine holding nothing.
type MemSample struct {
	Platform    string  `json:"platform"`
	FreeGB      float64 `json:"free_gb"`
	SwapUsedMB  float64 `json:"swap_used_mb"`
	TotalGB     float64 `json:"total_gb"`
	WiredGB     float64 `json:"wired_gb"`
	AnonymousGB float64 `json:"anonymous_gb"`

	// AvailableGB is Linux's MemAvailable: what the kernel estimates a new allocation can
	// take without swapping, reclaimable page cache included. macOS reports no equivalent.
	AvailableGB float64 `json:"available_gb"`

	OK bool `json:"-"` // false when the platform did not answer
}

// The platforms this project measures on, and the two headroom rules below.
const (
	platformDarwin = "darwin"
	platformLinux  = "linux"
)

// Headroom is what a sweep has left to take, and the arithmetic is per platform because
// the two kernels account for memory differently.
//
// macOS: total less wired and anonymous, which is what docs/TECH.md's envelope is stated
// in — Metal holds the model in wired memory and apps live in anonymous, while free memory
// sits near zero whichever state the machine is in.
//
// Linux: MemAvailable, the kernel's own estimate. Nothing there corresponds to the wired
// count the macOS rule subtracts, and MemFree would refuse every sweep because it excludes
// the page cache an allocation may reclaim.
//
// A sample that names no platform has no rule and reports nothing, which Check refuses:
// no number is not a big number.
func (s MemSample) Headroom() float64 {
	switch s.Platform {
	case platformDarwin:
		return s.TotalGB - s.WiredGB - s.AnonymousGB
	case platformLinux:
		return s.AvailableGB
	default:
		return 0
	}
}

// Sample is how a command reads this machine. A package variable so a test can hand the
// check a machine of its own, which is the only nondeterminism in the path.
var Sample = sampleMemory

// parseVMStat reads macOS `vm_stat` at the page size the machine reports. Each counter is
// keyed off its label: vm_stat's field positions differ by macOS version, and a shifted
// parse records a plausible number that is not the one the verdict rests on.
func parseVMStat(out string, page float64) MemSample {
	var s MemSample
	for _, line := range strings.Split(out, "\n") {
		var label string
		switch {
		case strings.HasPrefix(line, "Pages free:"):
			label = "Pages free:"
		case strings.HasPrefix(line, "Pages wired down:"):
			label = "Pages wired down:"
		case strings.HasPrefix(line, "Anonymous pages:"):
			label = "Anonymous pages:"
		default:
			continue
		}
		n, err := strconv.ParseFloat(strings.TrimSuffix(
			strings.TrimSpace(strings.TrimPrefix(line, label)), "."), 64)
		if err != nil {
			continue
		}
		gb := n * page / 1073741824
		switch label {
		case "Pages free:":
			s.FreeGB = gb
		case "Pages wired down:":
			s.WiredGB = gb
		case "Anonymous pages:":
			s.AnonymousGB = gb
		}
	}
	return s
}

// parseSwapUsage reads the used figure out of `sysctl -n vm.swapusage`, which prints
// "total = 4096.00M  used = 1465.44M  free = 2630.56M  (encrypted)". Keyed off the "used"
// label rather than field position: three numbers of the same shape sit on this line, and
// picking the wrong one records a plausible number that is not the one the run depended on.
func parseSwapUsage(out string) float64 {
	fields := strings.Fields(out)
	for i, f := range fields {
		if f != "used" {
			continue
		}
		for _, cand := range fields[i:min(i+3, len(fields))] {
			if !strings.HasSuffix(cand, "M") {
				continue
			}
			if n, err := strconv.ParseFloat(strings.TrimSuffix(cand, "M"), 64); err == nil {
				return n
			}
			break
		}
		break
	}
	return 0
}

// parseMeminfo reads Linux's /proc/meminfo, whose every line is "Name:<space>value kB".
// Keyed off the name for the same reason vm_stat is, and the unit column is not read: the
// kernel writes kB for every field taken here.
//
// A sample without MemTotal and MemAvailable is not a reading: MemAvailable is the whole
// headroom rule on Linux, so a file missing it leaves nothing to refuse a sweep with.
// Swap is total less free, which is what macOS reports directly.
func parseMeminfo(text string) MemSample {
	var s MemSample
	var swapTotalKB, swapFreeKB float64
	var haveTotal, haveAvailable bool
	for _, line := range strings.Split(text, "\n") {
		name, rest, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		kb, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}
		switch name {
		case "MemTotal":
			s.TotalGB, haveTotal = kb/1048576, true
		case "MemAvailable":
			s.AvailableGB, haveAvailable = kb/1048576, true
		case "MemFree":
			// Recorded because every row carries free memory, not because it is a
			// pressure signal — it is no more one here than on macOS.
			s.FreeGB = kb / 1048576
		case "SwapTotal":
			swapTotalKB = kb
		case "SwapFree":
			swapFreeKB = kb
		}
	}
	if !haveTotal || !haveAvailable {
		return MemSample{}
	}
	s.SwapUsedMB = (swapTotalKB - swapFreeKB) / 1024
	s.Platform, s.OK = platformLinux, true
	return s
}
