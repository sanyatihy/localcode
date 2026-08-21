package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Machine is every limit the hardware imposes, read from config/machine.json.
//
// It is config and not Go because these numbers are measurements of one laptop, and the
// vision calls a hardcoded machine limit a defect to fix rather than a value to update.
// Anything derived from the machine belongs here or in scripts/rungs.sh, which computes
// the rest from what the machine reports about itself.
type Machine struct {
	MinHeadroomGB float64       `json:"min_headroom_gb"`
	DeskProfiles  []DeskProfile `json:"desk_profiles"`
}

// LoadMachine reads it. Every field is required: a zero floor carries every sweep and
// reads as a machine with room, and a missing ceiling admits a context nobody measured.
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
	if len(m.DeskProfiles) == 0 {
		return Machine{}, fmt.Errorf("%s: no desk_profiles, so no context ceiling is known", path)
	}
	for _, p := range m.DeskProfiles {
		if p.Name == "" || p.Ceiling <= 0 {
			return Machine{}, fmt.Errorf("%s: desk profile %q needs a name and a positive ceiling_tokens", path, p.Name)
		}
	}
	return m, nil
}

// DeskProfile resolves a profile by name. There is no default: a tier-2 row that does not
// say which profile it was taken under cannot be compared with one that does.
func (m Machine) DeskProfile(name string) (DeskProfile, error) {
	known := make([]string, 0, len(m.DeskProfiles))
	for _, p := range m.DeskProfiles {
		if p.Name == name {
			return p, nil
		}
		known = append(known, p.Name)
	}
	return DeskProfile{}, fmt.Errorf("unknown desk profile %q; this machine declares %s",
		name, strings.Join(known, ", "))
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

// Check runs a sample against a headroom floor in GB. A sample the platform did not answer
// is refused: no number is not a big number.
func Check(s MemSample, floorGB float64) Preflight {
	h := s.Headroom()
	return Preflight{MemSample: s, HeadroomGB: h, FloorGB: floorGB, Carries: s.OK && h >= floorGB}
}

// Refuse reports why a sweep must not start, and "" when it may. The message names every
// number the verdict used, because a refusal a human cannot check is one they will force.
func (p Preflight) Refuse() string {
	if p.Carries {
		return ""
	}
	if !p.OK {
		return "the platform did not answer the memory probe, so headroom is unknown; -force starts anyway"
	}
	return fmt.Sprintf("%.2f GB headroom against a %.2f GB floor (%.2f GB free, %.0f MB swap in use); -force starts anyway",
		p.HeadroomGB, p.FloorGB, p.FreeGB, p.SwapUsedMB)
}
