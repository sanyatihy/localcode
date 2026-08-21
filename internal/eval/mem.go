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
	FreeGB      float64 `json:"free_gb"`
	SwapUsedMB  float64 `json:"swap_used_mb"`
	TotalGB     float64 `json:"total_gb"`
	WiredGB     float64 `json:"wired_gb"`
	AnonymousGB float64 `json:"anonymous_gb"`
	OK          bool    `json:"-"` // false when the platform did not answer
}

// Headroom is the arithmetic docs/TECH.md uses: what is left after wired and anonymous.
func (s MemSample) Headroom() float64 {
	return s.TotalGB - s.WiredGB - s.AnonymousGB
}

// Sample is how a command reads this machine. A package variable so a test can hand the
// check a machine of its own, which is the only nondeterminism in the path.
var Sample = sampleMemory

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
			// Each counter is keyed off its label: vm_stat's field positions differ by
			// macOS version, and a shifted parse records a plausible number that is not
			// the one the verdict rests on.
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
			s.OK = true
		}
	}
	if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
		if n, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil && n > 0 {
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
