package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sanyatihy/localcode/internal/eval"
)

// Hermes' floor and this machine's attended ceiling are two numbers measured apart from
// each other, and the whole shape of 0010's Hermes column follows from how they sit: the
// floor is above the ceiling with no overlap, so Hermes is admissible unattended only.
// Asserted rather than written down, because either number moving silently would put a
// Hermes row back in the attended table without anybody deciding to.
func TestHermesIsAdmissibleUnattendedOnly(t *testing.T) {
	h := NewHermes("../../harness/hermes/config.yaml.reference")
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
		// No served config: the profile's ceiling is the only bound in play, which is
		// the comparison this test is about.
		if admitted := p.Excludes(h, eval.ServerProps{}) == ""; admitted != tc.want {
			t.Errorf("%s (ceiling %d) admits hermes (floor %d) = %v, want %v",
				tc.profile, p.Ceiling, h.ContextFloor(), admitted, tc.want)
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

// Each run gets a home of its own holding nothing but the configuration. What makes this
// worth asserting is what is *absent*: the machine's Hermes carries sessions, memories and
// skills learned from earlier runs, and scoring against those measures how much Hermes has
// been used rather than how good it is.
func TestHermesBuildsAColdHome(t *testing.T) {
	h := NewHermes("../../harness/hermes/config.yaml.reference")
	home := t.TempDir()
	if err := h.seedHome(home); err != nil {
		t.Fatalf("seedHome: %v", err)
	}

	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "config.yaml" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("a cold home holds %v; it must hold the config and nothing else", names)
	}
	got, err := os.ReadFile(filepath.Join(home, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../harness/hermes/config.yaml.reference")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Error("the seeded config is not the committed one")
	}
}

// The committed config is what a run is produced by, so it has to name the local endpoint
// and a context Hermes will accept.
func TestTheCommittedHermesConfigServesTheLocalEndpoint(t *testing.T) {
	b, err := os.ReadFile("../../harness/hermes/config.yaml.reference")
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	for _, want := range []string{"provider: custom", "base_url: http://127.0.0.1:", "context_length:"} {
		if !strings.Contains(body, want) {
			t.Errorf("committed config no longer sets %q", want)
		}
	}
	// A config below the floor would be refused before a request is made, and the error
	// would name the context rather than the config that chose it.
	var ctx int
	if _, err := fmt.Sscanf(body[strings.Index(body, "context_length:"):], "context_length: %d", &ctx); err != nil {
		t.Fatalf("context_length unreadable: %v", err)
	}
	if ctx < HermesContextFloor {
		t.Errorf("committed config asks for %d, below Hermes' own floor of %d", ctx, HermesContextFloor)
	}
}
