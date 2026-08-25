package harness

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sanyatihy/localcode/internal/chain"
	"github.com/sanyatihy/localcode/internal/handoff"
)

// claudeCodeAgent runs a chain's sessions in Anthropic's claude CLI. It is configured
// entirely through environment variables and a settings file, which is why this holds a
// checkout: the committed environment is a file in it, and the hooks it installs are
// scripts in it.
type claudeCodeAgent struct {
	claudeCodeRecorder
	bin      string
	root     string // the localcode checkout the configuration is read from
	settings string // written by Prepare, read by every session of the chain
}

// claudeCodeRecorder is the half of the adapter that reads a finished session. It holds
// the environment because that is what says where the session filed its transcript; a
// reader that never ran one has none, and the default is where Claude Code puts it.
type claudeCodeRecorder struct {
	env []string // the committed environment, plus what Window settled
}

// The tool set 0008 settled, and the pre-approval that lets it run unattended. Both name
// the same four: --tools is the surface and --allowedTools is what makes it unprompted,
// and a session that has to ask writes the answer into the repository it is visiting.
const claudeCodeTools = "Bash,Edit,Read,Write"

func newClaudeCodeAgent(root string) (Agent, error) {
	bin, err := exec.LookPath("claude")
	if err != nil {
		return nil, fmt.Errorf("claude is not on PATH: %w", err)
	}
	env, err := EnvFromFile(ClaudeCodeEnvFile(root))
	if err != nil {
		return nil, err
	}
	return &claudeCodeAgent{claudeCodeRecorder: claudeCodeRecorder{env: env}, bin: bin, root: root}, nil
}

// ClaudeCodeEnvFile is the committed environment inside a checkout. Exported because it is
// also what says a directory is a localcode checkout at all.
func ClaudeCodeEnvFile(root string) string {
	return filepath.Join(root, "harness", "claude-code", "claude-code.env")
}

func (c *claudeCodeAgent) Name() string { return "claude-code" }

// Window declares the whole of what the server serves, and keeps the file's output
// reservation.
//
// Nothing is subtracted here. The harness holds back its own reservation from whatever it
// is told — measured, it will not send past about three quarters of the declaration less
// that reservation — so a declaration that has already taken one off leaves 4,096 tokens of
// the served context that neither the prompt nor the reply can ever use. The largest prompt
// it sent at a declared 49,152 was 34,008 tokens, which with the whole reservation on top is
// 38,104 of the 49,152 served: the declaration cannot overrun the server.
//
// It says so when the file disagrees, because the file is what an operator reading the
// configuration would believe, and a number silently overridden is a number nobody can
// account for afterwards.
func (c *claudeCodeAgent) Window(served int) (int, int, error) {
	fileContext, output, err := declared(c.env)
	if err != nil {
		return 0, 0, err
	}
	if served != fileContext {
		fmt.Fprintf(os.Stderr, "context: %d tokens, the whole of what the server serves — "+
			"claude-code.env declares %d, which is not this server\n", served, fileContext)
	}
	c.env = setEnv(c.env, "CLAUDE_CODE_MAX_CONTEXT_TOKENS", strconv.Itoa(served))
	// And the harness's own check on that number is off, because it is a second answer to
	// a question this repo has already answered. It holds back about a quarter of whatever
	// it is told — measured, it refuses past 34,258 tokens of a declared 49,152 — and the
	// fraction is undocumented, so a window derived from it is a window nobody can account
	// for. What it was protecting against is the server's own 400, and the gate is what
	// prevents that here: from a transcript reading, against a ceiling, with a handoff on
	// the other side of the denial. `claude-code.env` still leaves it on, because a session
	// somebody runs by hand has no gate.
	c.env = setEnv(c.env, "CLAUDE_CODE_DISABLE_UNKNOWN_MODEL_WINDOW_ENFORCEMENT", "1")
	return served, output, nil
}

// declared reads the window the harness was told it has. Both numbers are needed and
// neither is guessed: the budget is derived from them, and a guessed budget protects
// against a wall that is not the one there.
func declared(env []string) (maxContext, maxOutput int, err error) {
	read := func(name, value string, into *int) error {
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s is not a number: %q", name, value)
		}
		*into = n
		return nil
	}
	for _, kv := range env {
		name, value, _ := strings.Cut(kv, "=")
		switch name {
		case "CLAUDE_CODE_MAX_CONTEXT_TOKENS":
			err = read(name, value, &maxContext)
		case "CLAUDE_CODE_MAX_OUTPUT_TOKENS":
			err = read(name, value, &maxOutput)
		}
		if err != nil {
			return 0, 0, err
		}
	}
	if maxContext == 0 || maxOutput == 0 {
		return 0, 0, errors.New("claude-code.env must set both CLAUDE_CODE_MAX_CONTEXT_TOKENS " +
			"and CLAUDE_CODE_MAX_OUTPUT_TOKENS: a session's budget is derived from them")
	}
	return maxContext, maxOutput, nil
}

// setEnv replaces a variable rather than appending a second one. Two entries for one name
// leave which of them Claude Code reads to the C library, and the whole point of this one
// is that the session is held to the window it actually has.
func setEnv(env []string, name, value string) []string {
	entry := name + "=" + value
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, kv := range env {
		if k, _, _ := strings.Cut(kv, "="); k == name {
			if replaced {
				continue
			}
			kv, replaced = entry, true
		}
		out = append(out, kv)
	}
	if !replaced {
		out = append(out, entry)
	}
	return out
}

// Prepare renders the hooks with absolute paths. harness/claude-code/hooks.json addresses
// them through $CLAUDE_PROJECT_DIR, which is the repository being visited — so in anybody
// else's the hooks resolve to scripts that are not there and the session dies saying so.
// Generated rather than committed, because the path is only known once installed.
//
// The gate is this binary run as a hook rather than a fourth script. It reads a transcript
// and counts against a budget, which is the half of the repository `make check` covers.
//
// Stop is installed only for a session answering one instruction. It fires whenever the
// agent finishes responding, which in an interactive session is every time it hands the
// keyboard back — refusing there would refuse the conversation itself.
func (c *claudeCodeAgent) Prepare(chainDir string, oneShot bool) error {
	script := func(name string) any {
		return hookEntry(filepath.Join(c.root, "harness", "claude-code", "hooks", name))
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not find this binary to install it as a hook: %w", err)
	}
	hooks := map[string]any{
		"SessionStart": script("session-start.sh"),
		"PreCompact":   script("pre-compact.sh"),
		"SessionEnd":   script("session-end.sh"),
		"PreToolUse":   hookEntry(shellQuote(self) + " hook gate"),
	}
	if oneShot {
		hooks["Stop"] = hookEntry(shellQuote(self) + " hook stop")
	}
	body, err := json.MarshalIndent(map[string]any{"hooks": hooks}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(chainDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(chainDir, "settings.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("could not write %s: %w", path, err)
	}
	c.settings = path
	return nil
}

// hookEntry is one hook. Every matcher is empty, so each fires for everything its event
// covers.
func hookEntry(line string) any {
	return []any{map[string]any{"hooks": []any{map[string]string{
		"type":    "command",
		"command": line,
	}}}}
}

// shellQuote guards the one path here that a person did not type: the harness runs a hook
// through a shell, and an installation under a directory with a space in it would
// otherwise run the first word of it.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (c *claudeCodeAgent) Command(s Session) (string, []string, []string, error) {
	if c.settings == "" {
		return "", nil, nil, errors.New("claude-code: no settings file, so the session would " +
			"run with no gate: Prepare must run before a session does")
	}
	args := []string{
		"--tools", claudeCodeTools, "--allowedTools", claudeCodeTools,
		"--settings", c.settings,
		// The handoff is written outside the repository being visited, so a session leaves
		// it with exactly the files the work changed. Claude Code confines its file tools
		// to the workspace, and this is what puts that one directory in it.
		"--add-dir", s.StableDir,
		"--append-system-prompt", s.Briefing,
	}
	// The instruction goes last and verbatim. What the session before it learned arrives
	// separately, through the hook 0016 already uses, so a chain cannot drift by rewriting
	// its own goal at each hop.
	if s.Goal != "" {
		// Events rather than a result, because a result arrives once and this machine takes
		// minutes to reach it. The supervisor renders them, so it owns every line printed.
		args = append(args, "--output-format", "stream-json", "--verbose",
			"--include-partial-messages", "-p", s.Goal)
	}

	// A budget on calls is not a budget on tokens: one unbounded `cat` fills a window
	// inside a single permitted call, and the gate decides on the context as it stood
	// before that result arrived. This is the harness's own cap on the one tool whose
	// result has no bound of its own, sized from the window it is protecting.
	env := append(append([]string{}, c.env...),
		fmt.Sprintf("BASH_MAX_OUTPUT_LENGTH=%d", s.ResultCap),
		"LOCALCODE_HANDOFF_DIR="+s.StateDir)
	if s.Inherit != "" {
		env = append(env, "LOCALCODE_INHERIT="+s.Inherit)
	}
	return c.bin, args, env, nil
}

// Transcript finds what the session wrote about itself. Claude Code files a transcript
// under its config directory, keyed by the working directory, so this asks the environment
// the session ran with rather than guessing where that is.
func (c *claudeCodeRecorder) Transcript(dir string) string {
	// The session's own environment when there is one, and otherwise the environment this
	// reader is standing in: a chain read back long afterwards knows only that.
	env := c.env
	if env == nil {
		env = os.Environ()
	}
	config := ""
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "CLAUDE_CONFIG_DIR="); ok {
			config = v
		}
	}
	if config == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		config = filepath.Join(home, ".claude")
	}
	id := chain.SessionIDIn(dir)
	if id == "" {
		return ""
	}
	matches, err := filepath.Glob(filepath.Join(config, "projects", "*", id+".jsonl"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	return matches[0]
}

// Writable is the agent's own history and project state. One directory, and the sandbox
// denies everything else it might reach for.
func (c *claudeCodeAgent) Writable() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{filepath.Join(home, ".claude")}
}

// Cost and Requests are internal/handoff's readers: what a supervisor reports and what the
// gate enforces cannot drift apart if they are one count of one file.
func (c *claudeCodeRecorder) Cost(transcript string) (int, int) { return chain.Cost(transcript) }

func (c *claudeCodeRecorder) Requests(transcript string) ([]handoff.Request, error) {
	return handoff.Requests(transcript)
}

func (c *claudeCodeAgent) Render(events io.Reader, out io.Writer, cwd string) string {
	return renderStreamJSON(events, out, cwd)
}
