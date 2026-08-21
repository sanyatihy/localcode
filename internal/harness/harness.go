// Package harness wraps the coding agents this project can drive. Each is an external CLI
// with its own way of being pointed at a local endpoint, hidden behind one type here so
// nothing else in the repo knows those differences exist. harness/README.md holds what
// each one needs and which upstream project it is.
package harness

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sanyatihy/localcode/internal/eval"
)

// run executes a harness command. Shared so a non-zero exit always reports the tail of
// combined output rather than "exit status 1", and so the offline condition is applied in
// one place — no adapter can be scored offline by forgetting to.
func run(ctx context.Context, r eval.Run, name string, env []string, args ...string) error {
	// No clock of its own: the run's budget is the caller's, so that a harness stopped
	// for taking too long is recorded as over budget rather than as this adapter failing.
	name, args = sandboxed(r, name, args)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.Workdir
	// Trap: Dir does not update PWD, and Go replaces the environment wholesale when Env
	// is set, so a tool resolving its project from PWD works in the launching directory.
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
// unchanged when there is none. sandbox-exec is deprecated and still the only way to deny
// one process the network without touching the machine's; harness/offline.sb says why the
// denial has to be the kernel's.
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
