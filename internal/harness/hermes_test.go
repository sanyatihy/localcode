package harness

import (
	"testing"

	"github.com/sanyatihy/localcode/internal/eval"
)

// Hermes' floor and this machine's attended ceiling are two numbers measured apart from
// each other, and the whole shape of 0010's Hermes column follows from how they sit: the
// floor is above the ceiling with no overlap, so Hermes is admissible unattended only.
// Asserted rather than written down, because either number moving silently would put a
// Hermes row back in the attended table without anybody deciding to.
func TestHermesIsAdmissibleUnattendedOnly(t *testing.T) {
	h := NewHermes()
	if got := h.ContextFloor(); got != HermesContextFloor {
		t.Fatalf("floor = %d, want %d", got, HermesContextFloor)
	}
	for _, tc := range []struct {
		profile string
		want    bool
	}{
		{"attended", false},
		{"unattended", true},
	} {
		p, err := eval.LookupDeskProfile(tc.profile)
		if err != nil {
			t.Fatalf("LookupDeskProfile(%q): %v", tc.profile, err)
		}
		if admitted, floor := p.Admits(h); admitted != tc.want {
			t.Errorf("%s (ceiling %d) admits hermes (floor %d) = %v, want %v",
				tc.profile, p.Ceiling, floor, admitted, tc.want)
		}
	}
}

// The other three adapters declare no floor, so no profile can exclude them. A floor
// appearing on one of them is a change to what the comparison can say and must be a
// decision, not a side effect.
func TestOnlyHermesDeclaresAContextFloor(t *testing.T) {
	for _, d := range []eval.Driver{
		NewPi("../../harness/pi/local-provider.js", "local", "model"),
		NewOpenCode("../../harness/opencode/opencode.json", "local/model"),
		NewClaudeCode("../../harness/claude-code/claude-code.env"),
	} {
		if f, ok := d.(eval.ContextFloorer); ok {
			t.Errorf("%s declares a context floor of %d", d.Name(), f.ContextFloor())
		}
	}
}
