package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sanyatihy/localcode/internal/eval"
)

// Hermes drives NousResearch/hermes-agent. See harness/hermes/README.md for the
// configuration and harness/README.md for what it keeps between runs.
//
// Trap: a misconfigured Hermes reports "API call failed after 3 retries: Connection error"
// without opening a connection at all. That message means the config, not the network.
type Hermes struct {
	bin       string
	configRef string // committed config, copied into each run's home
}

// NewHermes returns a driver seeding each run's home from configRef. Without it Hermes
// falls back to built-in defaults and looks for a provider it has no endpoint for.
func NewHermes(configRef string) *Hermes { return &Hermes{bin: "hermes", configRef: configRef} }

func (h *Hermes) Name() string { return "hermes" }

// HermesContextFloor is Hermes' own refusal rather than a setting: it is checked in its
// code before any request, so lowering it means patching Hermes. Asserted here because
// upstream documents it nowhere.
const HermesContextFloor = 64000

func (h *Hermes) ContextFloor() int { return HermesContextFloor }

func (h *Hermes) Drive(ctx context.Context, r eval.Run) error {
	if err := h.seedHome(r.StateDir); err != nil {
		return err
	}
	if err := run(ctx, r, h.bin, append(os.Environ(), "HERMES_HOME="+r.StateDir),
		"--yolo", // auto-approve tools; the scratch dir is disposable
		"--cli",
		"--in", r.Workdir,
		"-z", r.Instruction,
	); err != nil {
		return err
	}
	// The runner's own state-directory check passes on the config seeded above, so a
	// Hermes that ignored HERMES_HOME — and ran against the machine's accumulated
	// skills and memories instead — has to be caught here.
	if _, err := os.Stat(filepath.Join(r.StateDir, "state.db")); err != nil {
		return fmt.Errorf("hermes left no state under HERMES_HOME=%s, so the run was not cold", r.StateDir)
	}
	return nil
}

// seedHome writes the committed configuration into an otherwise empty home. Hermes fills
// in the rest on first use.
func (h *Hermes) seedHome(home string) error {
	cfg, err := os.ReadFile(h.configRef)
	if err != nil {
		return fmt.Errorf("hermes config not readable: %w", err)
	}
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), cfg, 0o644); err != nil {
		return fmt.Errorf("seed hermes config: %w", err)
	}
	return nil
}
