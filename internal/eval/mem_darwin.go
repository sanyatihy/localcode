package eval

import (
	"os/exec"
	"strconv"
	"strings"
)

// pageSize is read, never assumed. It was hardcoded to 4096 once, on a machine that pages
// at 16384, and every free-memory figure recorded before that was found was four times
// too small.
func pageSize() float64 {
	out, err := exec.Command("pagesize").Output()
	if err != nil {
		return 0
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// sampleMemory reads this Mac: the page counters off vm_stat, the installed memory and the
// swap in use off sysctl. A sample missing any of the three figures the macOS headroom rule
// needs is returned empty rather than partial, so a preflight refuses it instead of
// subtracting a zero that reads as a machine holding nothing.
func sampleMemory() MemSample {
	page := pageSize()
	if page == 0 {
		return MemSample{}
	}
	var s MemSample
	if out, err := exec.Command("vm_stat").Output(); err == nil {
		s = parseVMStat(string(out), page)
	}
	if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
		if n, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil && n > 0 {
			s.TotalGB = n / 1073741824
		}
	}
	if out, err := exec.Command("sysctl", "-n", "vm.swapusage").Output(); err == nil {
		s.SwapUsedMB = parseSwapUsage(string(out))
	}
	if s.TotalGB <= 0 || s.WiredGB <= 0 || s.AnonymousGB <= 0 {
		return MemSample{}
	}
	s.Platform, s.OK = platformDarwin, true
	return s
}
