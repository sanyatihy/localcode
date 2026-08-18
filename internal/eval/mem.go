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
	OK         bool    `json:"-"` // false when the platform did not answer
}

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
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.HasPrefix(line, "Pages free:") {
				continue
			}
			f := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, "Pages free:")), ".")
			if n, err := strconv.ParseFloat(f, 64); err == nil {
				s.FreeGB = n * page / 1073741824
				s.OK = true
			}
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
