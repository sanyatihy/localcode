package eval

import (
	"os/exec"
	"strconv"
	"strings"
)

// ThermalSample is the machine's own account of whether it was allowed to run at full
// speed. A timing taken while the scheduler was capped measures the cap, which is the
// same category of void as a run that swapped: not slow, but measuring something else.
//
// The reading is cumulative in the way swap is. macOS records a thermal warning when one
// occurs and leaves it there, so an absolute value says only that the machine has been
// throttled at some point since boot. The signal is the delta across a run, which is why
// this is sampled either side of one rather than read once.
type ThermalSample struct {
	// SpeedLimit is the percentage of full speed the CPU is permitted, 100 when
	// unthrottled. A machine that has never been warned reports nothing at all, which
	// is the same state and is recorded as 100.
	SpeedLimit int  `json:"speed_limit"`
	OK         bool `json:"-"` // false when the platform did not answer
}

// sampleThermal reads `pmset -g therm`, which needs no privileges. powermetrics reports
// more and requires sudo, and a measurement gate that cannot run without a password is a
// gate that will be skipped.
func sampleThermal() ThermalSample {
	out, err := exec.Command("pmset", "-g", "therm").Output()
	if err != nil {
		return ThermalSample{}
	}
	return parseThermal(string(out))
}

func parseThermal(s string) ThermalSample {
	sample := ThermalSample{SpeedLimit: 100, OK: true}
	for _, line := range strings.Split(s, "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found || !strings.Contains(key, "CPU_Speed_Limit") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return ThermalSample{}
		}
		sample.SpeedLimit = n
	}
	return sample
}

// Throttled reports whether the machine was capped across a run, and by how much. A run
// that started capped is throttled too: what is being asked is whether the timing can be
// trusted, not whether the cap arrived during it.
func Throttled(before, after ThermalSample) (throttled bool, limit int) {
	if !before.OK || !after.OK {
		return false, 0
	}
	limit = before.SpeedLimit
	if after.SpeedLimit < limit {
		limit = after.SpeedLimit
	}
	return limit < 100, limit
}
