package eval

import (
	"encoding/json"
	"fmt"
	"os"
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

// Machine is what config/ records about the hardware, kept there rather than in Go because
// a floor derived on 32 GB is wrong on the next machine and would be re-derived by hand.
type Machine struct {
	MinHeadroomGB float64 `json:"min_headroom_gb"`
}

// LoadMachine reads it. A file that names no floor is an error rather than a zero, which
// would carry every sweep and read as a machine with room.
func LoadMachine(path string) (Machine, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Machine{}, err
	}
	var m Machine
	if err := json.Unmarshal(b, &m); err != nil {
		return Machine{}, fmt.Errorf("%s: %w", path, err)
	}
	if m.MinHeadroomGB <= 0 {
		return Machine{}, fmt.Errorf("%s: min_headroom_gb must be above zero", path)
	}
	return m, nil
}

// Preflight reports whether this machine can carry a sweep, with the numbers it read.
// Headroom is total less wired and anonymous — what competes for the RAM — because free
// memory is no pressure signal on macOS (see MemSample) and a floor on it would refuse
// every sweep this project runs.
type Preflight struct {
	MemSample
	HeadroomGB float64 `json:"headroom_gb"`
	FloorGB    float64 `json:"floor_gb"`
	Carries    bool    `json:"carries"`
}

// Headroom is the arithmetic docs/TECH.md uses: what is left after wired and anonymous.
func (s MemSample) Headroom() float64 {
	return s.TotalGB - s.WiredGB - s.AnonymousGB
}

// Check runs a sample against a headroom floor in GB. A floor of 0 refuses nothing, and
// a sample the platform did not answer is refused: no number is not a big number.
func Check(s MemSample, floorGB float64) Preflight {
	h := s.Headroom()
	return Preflight{MemSample: s, HeadroomGB: h, FloorGB: floorGB, Carries: s.OK && h >= floorGB}
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
