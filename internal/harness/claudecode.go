package harness

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sanyatihy/localcode/internal/eval"
)

// ClaudeCode drives Anthropic's claude CLI against the local endpoint. It is the only
// harness here that speaks the Anthropic Messages API, and the only one configured
// entirely through environment variables — which is why its configuration is a file this
// adapter reads rather than a flag. It also needs the server configured for it: see
// harness/claude-code/README.md.
type ClaudeCode struct {
	bin     string
	envFile string // committed environment file; every variable in it is documented
	tools   string
}

// NewClaudeCode returns a driver reading its configuration from envFile. The tool set is
// chosen, not accepted: all 21 tools cost 18,388 tokens of preamble against 3,711 for
// these three, and Bash is withheld because this repo runs the tests that score the run.
func NewClaudeCode(envFile string) *ClaudeCode {
	return &ClaudeCode{bin: "claude", envFile: envFile, tools: "Read,Edit,Write"}
}

func (c *ClaudeCode) Name() string { return "claude-code" }

func (c *ClaudeCode) Drive(ctx context.Context, r eval.Run) error {
	env, err := EnvFromFile(c.envFile)
	if err != nil {
		return err
	}
	// History, project state and any user-level instructions live under this directory.
	// Pointed at a fresh one, the session starts with nothing the machine has learned.
	env = append(env, "CLAUDE_CONFIG_DIR="+r.StateDir)
	return run(ctx, r, c.bin, env,
		"-p", // non-interactive: process the prompt and exit
		"--tools", c.tools,
		"--permission-mode", "acceptEdits", // the scratch checkout is disposable
		r.Instruction,
	)
}

// EnvFromFile builds a child environment from the committed file, dropping every
// ANTHROPIC_* and CLAUDE_* entry the parent carries. Dropping them is the point: a stray
// ANTHROPIC_API_KEY outranks the file's credential, and a session launched from inside
// Claude Code exports CLAUDE_CODE_* variables that change the tool set — one such
// environment produced 21 tools where a clean one produced 18.
func EnvFromFile(path string) ([]string, error) {
	vars, err := parseEnvFile(path)
	if err != nil {
		return nil, err
	}
	var env []string
	for _, kv := range os.Environ() {
		if name, _, ok := strings.Cut(kv, "="); ok && isAgentVar(name) {
			continue
		}
		env = append(env, kv)
	}
	return append(env, vars...), nil
}

func isAgentVar(name string) bool {
	return strings.HasPrefix(name, "ANTHROPIC_") ||
		strings.HasPrefix(name, "CLAUDE_") ||
		name == "CLAUDECODE"
}

// parseEnvFile reads the shell-sourceable KEY="value" form this repo's config files use.
// Strict on purpose: a silently dropped variable serves a different configuration under
// the same label.
func parseEnvFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("claude-code environment not readable: %w", err)
	}
	defer func() { _ = f.Close() }()

	var out []string
	scan := bufio.NewScanner(f)
	for line := 1; scan.Scan(); line++ {
		text := strings.TrimSpace(scan.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		name, value, ok := strings.Cut(text, "=")
		if !ok || name == "" {
			return nil, fmt.Errorf("%s:%d: not KEY=value: %q", path, line, text)
		}
		out = append(out, name+"="+unquote(value))
	}
	if err := scan.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s sets nothing", path)
	}
	return out, nil
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
