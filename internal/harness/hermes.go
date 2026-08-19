package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sanyatihy/localcode/internal/eval"
)

// Hermes drives NousResearch/hermes-agent.
//
// Two things make it unlike the others. It keeps everything in one home directory —
// sessions, memories, skills learned from past runs — and that home is `~/.hermes` unless
// `HERMES_HOME` says otherwise, so a run inherits whatever the machine's Hermes has picked
// up. This adapter gives each run a home of its own, seeded from the config the repo
// commits: what it scores is a Hermes that has never run before, and the machine's own
// home is neither read nor written. And it refuses any context window below 64,000 tokens,
// checked before a request is made — so a server on the 32k baseline fails here with a
// message about context, while Pi and OpenCode run against it happily.
//
// A misconfigured Hermes reports "API call failed after 3 retries: Connection error"
// without opening a connection at all, so that message means "check the config", not
// "check the network".
type Hermes struct {
	bin       string
	configRef string // committed config, copied into each run's home
}

// NewHermes returns a driver seeding each run's home from configRef. Without it Hermes
// falls back to built-in defaults and looks for a provider it has no endpoint for.
func NewHermes(configRef string) *Hermes { return &Hermes{bin: "hermes", configRef: configRef} }

func (h *Hermes) Name() string { return "hermes" }

// HermesContextFloor is the smallest context window Hermes accepts. It is a check in its
// own code and not a setting: `context_length` in ~/.hermes/config.yaml selects what it
// asks for, and anything below this is refused before a request is made, so lowering it
// means patching Hermes — which is then a different harness and scored under its own name.
//
// 0014 put this machine's attended ceiling at 57,344, so the floor sits above it with no
// overlap: Hermes is admissible unattended only.
const HermesContextFloor = 64000

func (h *Hermes) ContextFloor() int { return HermesContextFloor }

func (h *Hermes) Drive(ctx context.Context, r eval.Run) error {
	if err := h.seedHome(r.StateDir); err != nil {
		return err
	}
	if err := run(ctx, h.bin, r.Workdir, append(os.Environ(), "HERMES_HOME="+r.StateDir),
		"--yolo", // auto-approve tools; the scratch dir is disposable
		"--cli",
		"--in", r.Workdir,
		"-z", r.Instruction,
	); err != nil {
		return err
	}
	// Checked here and not by the runner's own state-directory check, which this adapter
	// would satisfy with the config it seeds. A Hermes that ignored HERMES_HOME used the
	// machine's home instead, and that one carries every skill and memory it has
	// accumulated — the confound the arrangement exists to remove.
	if _, err := os.Stat(filepath.Join(r.StateDir, "state.db")); err != nil {
		return fmt.Errorf("hermes left no state under HERMES_HOME=%s, so the run was not cold", r.StateDir)
	}
	return nil
}

// seedHome writes the committed configuration into an otherwise empty home: no sessions,
// no memories, no learned skills. Hermes fills in the rest on first use.
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
