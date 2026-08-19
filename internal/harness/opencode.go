package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sanyatihy/localcode/internal/eval"
)

// OpenCode drives anomalyco/opencode. Its provider is declared in an opencode.json that
// must sit in the working directory, so Drive copies the committed config into each
// scratch checkout rather than relying on global state.
//
// Note this is not opencode-ai/opencode, which is an archived predecessor under a
// different owner — the same name, a different project.
type OpenCode struct {
	bin    string
	config string // committed opencode.json
	model  string // provider/model, e.g. local/<served model id>
}

func NewOpenCode(config, model string) *OpenCode {
	return &OpenCode{bin: "opencode", config: config, model: model}
}

func (o *OpenCode) Name() string { return "opencode" }

func (o *OpenCode) Drive(ctx context.Context, r eval.Run) error {
	data, err := os.ReadFile(o.config)
	if err != nil {
		return fmt.Errorf("opencode config not readable: %w", err)
	}
	if err := os.WriteFile(filepath.Join(r.Workdir, "opencode.json"), data, 0o644); err != nil {
		return fmt.Errorf("place opencode config: %w", err)
	}
	// Sessions live in an SQLite database under XDG_DATA_HOME, and every scratch
	// checkout lands in one project called "global" — so without this each run would
	// share a database with the last. The cache is deliberately left alone: it holds
	// provider metadata that is fetched, not learned.
	env := append(os.Environ(), "XDG_DATA_HOME="+r.StateDir)
	return run(ctx, r, o.bin, env, "run", "-m", o.model, r.Instruction)
}
