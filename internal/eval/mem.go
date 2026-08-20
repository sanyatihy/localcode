package eval

import (
	"os/exec"
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
type MemSample struct {
	FreeGB     float64 `json:"free_gb"`
	SwapUsedMB float64 `json:"swap_used_mb"`

	// Wired plus anonymous is what competes for the machine's memory; file-backed pages
	// are evictable and do not. docs/TECH.md derives the ceiling from these two.
	WiredGB     float64 `json:"wired_gb"`
	AnonymousGB float64 `json:"anonymous_gb"`
	TotalGB     float64 `json:"total_gb"`

	OK bool `json:"-"` // false when the platform did not answer
}

// HeadroomGB is what is left for anything not already resident.
func (s MemSample) HeadroomGB() float64 { return s.TotalGB - s.WiredGB - s.AnonymousGB }

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

func sampleMemory() MemSample {
	page := pageSize()
	if page == 0 {
		return MemSample{}
	}
	var s MemSample
	if out, err := exec.Command("vm_stat").Output(); err == nil {
		pages := map[string]*float64{
			"Pages free:":       &s.FreeGB,
			"Pages wired down:": &s.WiredGB,
			"Anonymous pages:":  &s.AnonymousGB,
		}
		for _, line := range strings.Split(string(out), "\n") {
			for prefix, field := range pages {
				if !strings.HasPrefix(line, prefix) {
					continue
				}
				f := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, prefix)), ".")
				if n, err := strconv.ParseFloat(f, 64); err == nil {
					*field = n * page / 1073741824
					s.OK = true
				}
			}
		}
	}
	if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
		if n, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil {
			s.TotalGB = n / 1073741824
		}
	}
	if out, err := exec.Command("sysctl", "-n", "vm.swapusage").Output(); err == nil {
		// "total = 4096.00M  used = 1465.44M  free = 2630.56M  (encrypted)". Keyed off the
		// "used" label rather than field position: three numbers of the same shape sit on
		// this line, and picking the wrong one records a plausible number that is not the
		// one the run depended on.
		fields := strings.Fields(string(out))
		for i, f := range fields {
			if f != "used" {
				continue
			}
			for _, cand := range fields[i:min(i+3, len(fields))] {
				if !strings.HasSuffix(cand, "M") {
					continue
				}
				if n, err := strconv.ParseFloat(strings.TrimSuffix(cand, "M"), 64); err == nil {
					s.SwapUsedMB, s.OK = n, true
				}
				break
			}
			break
		}
	}
	return s
}
