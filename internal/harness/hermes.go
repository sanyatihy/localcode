package harness

import (
	"context"
	"os"
)

// Hermes drives NousResearch/hermes-agent.
//
// Two things make it unlike the others. Its configuration is global machine state in
// ~/.hermes/config.yaml with no project-local form, so this adapter cannot carry it and
// the repo keeps only a reference copy. And it refuses any context window below 64,000
// tokens, checked before a request is made — so a server on the 32k baseline fails here
// with a message about context, while Pi and OpenCode run against it happily.
//
// A misconfigured Hermes reports "API call failed after 3 retries: Connection error"
// without opening a connection at all, so that message means "check the config", not
// "check the network".
type Hermes struct {
	bin string
}

func NewHermes() *Hermes { return &Hermes{bin: "hermes"} }

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

func (h *Hermes) Drive(ctx context.Context, workdir, instruction string) error {
	return run(ctx, h.bin, workdir, os.Environ(),
		"--yolo", // auto-approve tools; the scratch dir is disposable
		"--cli",
		"--in", workdir,
		"-z", instruction,
	)
}
