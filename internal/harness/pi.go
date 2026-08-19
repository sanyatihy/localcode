package harness

import (
	"context"
	"fmt"
	"os"

	"github.com/sanyatihy/localcode/internal/eval"
)

// Pi drives earendil-works/pi. Its provider is registered by an extension file loaded
// with -e, so the configuration travels with the repo and a run does not depend on a
// machine having been set up by hand.
type Pi struct {
	bin       string
	extension string // path to the provider-registering extension
	provider  string
	model     string
	apiKey    string
	tools     string
}

// NewPi returns a driver for pi. extension must point at the committed provider file;
// without it pi resolves the model against api.openai.com and returns 401.
func NewPi(extension, provider, model string) *Pi {
	return &Pi{
		bin:       "pi",
		extension: extension,
		provider:  provider,
		model:     model,
		apiKey:    "local", // llama-server checks nothing; pi requires the field to exist
		// bash is withheld deliberately: the fixture is scored by tests this repo runs,
		// so a harness running its own is spending turns without adding signal.
		tools: "read,edit,write",
	}
}

func (p *Pi) Name() string { return "pi" }

func (p *Pi) Drive(ctx context.Context, r eval.Run) error {
	if _, err := os.Stat(p.extension); err != nil {
		return fmt.Errorf("pi extension not readable: %w", err)
	}
	env := append(os.Environ(), "LOCAL_OPENAI_API_KEY="+p.apiKey)
	return run(ctx, r, p.bin, env,
		"-p", // non-interactive: process the prompt and exit
		"-e", p.extension,
		"--provider", p.provider,
		"--model", p.model,
		"--tools", p.tools,
		// Sessions are keyed by working directory under ~/.pi by default. A scratch
		// checkout is new every run, so nothing could carry over — but that is a
		// property of the fixture runner rather than of pi, and this makes it pi's.
		"--session-dir", r.StateDir,
		r.Instruction,
	)
}
