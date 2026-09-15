package eval

import "os/exec"

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
