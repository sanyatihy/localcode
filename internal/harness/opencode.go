package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

func (o *OpenCode) Drive(ctx context.Context, workdir, instruction string) error {
	data, err := os.ReadFile(o.config)
	if err != nil {
		return fmt.Errorf("opencode config not readable: %w", err)
	}
	if err := os.WriteFile(filepath.Join(workdir, "opencode.json"), data, 0o644); err != nil {
		return fmt.Errorf("place opencode config: %w", err)
	}
	return run(ctx, o.bin, workdir, os.Environ(), "run", "-m", o.model, instruction)
}
