package eval

import "os"

// sampleMemory reads this box: one file, no subprocesses. /proc/meminfo is the kernel's own
// account and needs no privileges; the macOS probes have no counterpart here and record
// unknown rather than a number, which is what the Arm Linux gate holds them to.
func sampleMemory() MemSample {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return MemSample{}
	}
	return parseMeminfo(string(b))
}
