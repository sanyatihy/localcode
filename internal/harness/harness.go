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
	"time"
)

// DefaultTimeout bounds a single harness run. Generous because a 64k cold ingest alone
// measured 13.1 minutes, and a harness that is merely slow must not be recorded as broken.
const DefaultTimeout = 45 * time.Minute

// run executes a harness command and returns a useful error. Shared because all three
// adapters need identical treatment of a non-zero exit: the tail of combined output, not
// "exit status 1", which tells nobody anything.
func run(ctx context.Context, name, dir string, env []string, args ...string) error {
	runCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, name, args...)
	cmd.Dir = dir
	// Setting Dir does not update PWD, and Go replaces the whole environment when Env
	// is set. Tools that resolve their project from PWD rather than getcwd() then look
	// in the directory this process was launched from — for OpenCode that surfaces as
	// "Unexpected server error", which reproduces through an adapter and never by hand.
	cmd.Env = withPWD(env, dir)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			return fmt.Errorf("%s timed out after %s", name, DefaultTimeout)
		}
		return fmt.Errorf("%s: %w: %s", name, err, tail(out.String(), 300))
	}
	return nil
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
