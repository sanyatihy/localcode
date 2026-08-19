// Package harness wraps the coding agents this project can drive. Each agent is an
// external CLI with its own way of being pointed at a local endpoint, and each is hidden
// behind one type here so nothing else in the repo knows those differences exist.
//
// Only Claude Code accepts a local endpoint through an environment variable. Pi ignores
// OPENAI_BASE_URL and calls api.openai.com; OpenCode ignores LOCAL_ENDPOINT. Both claims
// circulate online and neither is true, which is why the configuration lives in the repo
// rather than in a README instruction.
package harness

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sanyatihy/localcode/internal/eval"
)

// run executes a harness command and returns a useful error. Shared because all four
// adapters need identical treatment of a non-zero exit: the tail of combined output, not
// "exit status 1", which tells nobody anything — and because the offline condition is
// applied here, so no adapter can be scored offline by forgetting to.
func run(ctx context.Context, r eval.Run, name string, env []string, args ...string) error {
	// No clock of its own: the run's budget is the caller's, so that a harness stopped
	// for taking too long is recorded as over budget rather than as this adapter failing.
	name, args = sandboxed(r, name, args)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.Workdir
	// Setting Dir does not update PWD, and Go replaces the whole environment when Env
	// is set. Tools that resolve their project from PWD rather than getcwd() then look
	// in the directory this process was launched from — for OpenCode that surfaces as
	// "Unexpected server error", which reproduces through an adapter and never by hand.
	cmd.Env = withPWD(env, r.Workdir)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%s was stopped: %w", name, ctx.Err())
		}
		return fmt.Errorf("%s: %w: %s", name, err, tail(out.String(), 300))
	}
	return nil
}

// sandboxed wraps a command in the sandbox profile the run names, and returns it
// unchanged when there is none.
//
// sandbox-exec is deprecated and still the only way to deny one process the network
// without touching the machine's. The denial is the kernel's: a harness that ignores
// proxy variables cannot be recorded as working offline while it was online the whole
// time, which is the failure mode an environment-variable block has.
func sandboxed(r eval.Run, name string, args []string) (string, []string) {
	if r.SandboxProfile == "" {
		return name, args
	}
	return "sandbox-exec", append([]string{"-f", r.SandboxProfile, name}, args...)
}

// withPWD replaces any PWD entry so it agrees with the working directory.
func withPWD(env []string, dir string) []string {
	if dir == "" {
		return env
	}
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if !strings.HasPrefix(kv, "PWD=") {
			out = append(out, kv)
		}
	}
	return append(out, "PWD="+dir)
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
