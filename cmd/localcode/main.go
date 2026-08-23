// Command localcode drives the local model from whatever repository the developer is
// standing in. It is the only thing here meant to be installed rather than run from this
// checkout, which is why it carries that checkout's path instead of looking for one.
//
// It writes nothing to the repository it runs in. The tool set and the pre-approval come
// from 0008, and the session's own state goes to a directory of its own, so a visited
// repository ends a session with exactly the files the work changed.
//
// Exit codes are the contract:
//
//	0  the chain finished the instruction, the session exited cleanly, or the subcommand
//	   answered yes
//	1  the chain stopped without finishing — its bound, a stall, its clock or a Ctrl-C —
//	   or the session exited non-zero, or the subcommand answered no
//	2  the command could not be carried out (no checkout, no server, bad flags, a window
//	   too small to work in)
//
// A chain that stops on 1 has left a handoff and names it, so `-resume` carries it on.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sanyatihy/localcode/internal/chain"
	"github.com/sanyatihy/localcode/internal/harness"
)

// checkout is the localcode checkout this binary was built from, stamped in by
// `make install`. Nothing searches for it: a launcher that guesses which checkout it
// belongs to picks the wrong one as soon as there are two.
var checkout = ""

// The tool set 0008 settled, and the pre-approval that lets it run unattended. Both name
// the same four: --tools is the surface and --allowedTools is what makes it unprompted,
// and a session that has to ask writes the answer into the repository it is visiting.
const agentTools = "Bash,Edit,Read,Write"

const usage = `localcode — drive the local model in this repository

usage:
  localcode [flags] [prompt]   run the agent here
  localcode serve              start the server here, in the foreground
  localcode stop               stop it, waiting for the memory back
  localcode status             what is being served, if anything
  localcode sessions           the chains this repository has run
  localcode hook <name>        run one of this binary's own session hooks
  localcode --help

flags:
  -checkout dir      the localcode checkout to read configuration from
  -endpoint url      the server to use
  -config file       the serving config to start (default config/agent.env)
  -no-serve          refuse if no server is running, rather than starting one
  -net               allow outbound network for this session (loopback only by default)
  -ceiling pct       lower the ceiling below what the reserve already allows
  -calls n           how many tool calls a session may spend
  -sessions n        how many sessions one instruction may take
  -session-timeout d how long one session may run before it is stopped
  -continue          carry on this repository's most recent chain
  -resume id         carry on the chain with this id
  -fork id           start a chain from what that chain knew
`

func main() {
	fs := flag.NewFlagSet("localcode", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	checkoutFlag := fs.String("checkout", "", "the localcode checkout to read configuration from")
	endpoint := fs.String("endpoint", "http://127.0.0.1:8081", "the server to use")
	config := fs.String("config", "config/agent.env", "the serving config to start")
	noServe := fs.Bool("no-serve", false, "refuse if no server is running")
	net := fs.Bool("net", false, "allow outbound network for this session")
	ceiling := fs.Int("ceiling", 100, "how much of the window a session may fill, in percent")
	calls := fs.Int("calls", 30, "how many tool calls a session may spend")
	sessions := fs.Int("sessions", 8, "how many sessions one instruction may take")
	timeout := fs.Duration("session-timeout", 30*time.Minute, "how long one session may run")
	cont := fs.Bool("continue", false, "carry on this repository's most recent chain")
	resume := fs.String("resume", "", "carry on the chain with this id")
	fork := fs.String("fork", "", "start a chain from what the chain with this id knew")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	args := fs.Args()
	var (
		code int
		err  error
	)
	switch {
	case len(args) > 0 && args[0] == "status":
		code, err = status(*endpoint)
	case len(args) > 0 && args[0] == "serve":
		code, err = script(*checkoutFlag, "serve.sh", *config)
	case len(args) > 0 && args[0] == "stop":
		code, err = script(*checkoutFlag, "stop.sh")
	case len(args) > 1 && args[0] == "hook":
		code, err = hook(args[1])
	case len(args) > 0 && args[0] == "sessions":
		code, err = sessionsHere()
	default:
		code, err = run(opts{
			checkout: *checkoutFlag,
			endpoint: *endpoint,
			config:   *config,
			noServe:  *noServe,
			net:      *net,
			ceiling:  *ceiling,
			calls:    *calls,
			sessions: *sessions,
			timeout:  *timeout,
			cont:     *cont,
			resume:   *resume,
			fork:     *fork,
			args:     args,
		})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "localcode: "+err.Error())
	}
	os.Exit(code)
}

type opts struct {
	checkout string
	endpoint string
	config   string
	noServe  bool
	net      bool
	ceiling  int
	calls    int
	sessions int
	timeout  time.Duration
	cont     bool
	resume   string
	fork     string
	args     []string
}

func run(o opts) (int, error) {
	root, err := resolveCheckout(o.checkout)
	if err != nil {
		return 2, err
	}
	if err := ensureServer(root, o.endpoint, o.config, o.noServe); err != nil {
		return 2, err
	}

	env, err := harness.EnvFromFile(filepath.Join(root, "harness", "claude-code", "claude-code.env"))
	if err != nil {
		return 2, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return 2, fmt.Errorf("no working directory: %w", err)
	}

	// The budget is derived from the window the harness was declared, so a served config
	// and the enforcement over it cannot disagree. Refused rather than guessed: a session
	// started in a window nothing fits in spends a cold ingest to say `Prompt is too long`.
	maxContext, maxOutput, err := declared(env)
	if err != nil {
		return 2, err
	}
	limits, err := chain.NewLimits(maxContext, maxOutput, o.ceiling, o.calls)
	if err != nil {
		return 2, err
	}

	// Sessions are kept out of the repository being visited, which is what lets the
	// handoff hooks run in somebody else's checkout at all.
	state, err := repoState(cwd)
	if err != nil {
		return 2, err
	}
	chainDir, id, goal, err := selectChain(state, o)
	if err != nil {
		return 2, err
	}

	// Whether there is an instruction decides more than what the harness is told. A
	// session answering one ends once; an interactive session ends every time it hands the
	// keyboard back. Only the first can be chained, and only the first can be refused
	// permission to stop without leaving a handoff.
	oneShot := goal != ""
	settings, err := writeSettings(root, chainDir, oneShot)
	if err != nil {
		return 2, err
	}

	// The agent runs inside a seatbelt sandbox, which is what makes exposing an
	// unrestricted Bash tool defensible: the boundary is the kernel's rather than the
	// model's judgement, and it needs to know nothing about the language in the repository.
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return 2, fmt.Errorf("claude is not on PATH: %w", err)
	}
	// Refused rather than skipped: running unsandboxed because the sandbox is missing is
	// the one failure mode that would be silent and would matter.
	if _, err := os.Stat(sandboxExec); err != nil {
		return 2, fmt.Errorf("no %s: localcode runs the agent sandboxed and will not run it otherwise", sandboxExec)
	}
	if o.net {
		fmt.Fprintln(os.Stderr, "network: outbound ENABLED for this session")
	}
	profile, err := writeSandboxProfile(state, cwd, o.net)
	if err != nil {
		return 2, err
	}

	// A budget on calls is not a budget on tokens: one unbounded `cat` fills a window
	// inside a single permitted call, and the gate decides on the context as it stood
	// before that result arrived. This is the harness's own cap on the one tool whose
	// result has no bound of its own, sized from the window it is protecting.
	env = append(env, fmt.Sprintf("BASH_MAX_OUTPUT_LENGTH=%d", limits.ResultCap))

	l := launch{
		claude: claudePath, sandbox: sandboxExec, profile: profile, settings: settings,
		env: env, limits: limits, briefing: sandboxBriefing(cwd), cwd: cwd,
	}
	if oneShot {
		// The clock is the supervisor's bound on a session nobody is watching.
		l.timeout = o.timeout
		return runChain(l, chainDir, id, goal, o.sessions)
	}

	// With no instruction this is a developer at a keyboard, so the session is one, what
	// follows it is their decision rather than a loop's, and it ends when they end it —
	// a timer that stopped a session somebody was using would be the tool's worst bug.
	n := chain.NextSession(chainDir)
	dir := filepath.Join(chainDir, fmt.Sprintf("%02d", n))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 2, err
	}
	r, err := l.session(dir, n, id, "", chain.LatestHandoff(chainDir))
	if err != nil {
		return 2, err
	}
	if r.Handoff > 0 {
		narrate("\nhanded off in %s — carry on with `localcode -continue`\n",
			filepath.Join(dir, chain.HandoffName))
	}
	return r.Exit, nil
}

// sessionsHere lists what this repository has been asked to do. Keyed by the repository,
// because two checkouts of one project are the normal case here and they are not the same
// box of work.
func sessionsHere() (int, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return 2, fmt.Errorf("no working directory: %w", err)
	}
	state, err := repoState(cwd)
	if err != nil {
		return 2, err
	}
	return listChains(state)
}

// resolveCheckout prefers the flag, then the stamped path. It refuses rather than
// searching: the error names the one command that fixes it, which a guess cannot.
func resolveCheckout(flagValue string) (string, error) {
	for _, candidate := range []string{flagValue, checkout} {
		if candidate == "" {
			continue
		}
		envFile := filepath.Join(candidate, "harness", "claude-code", "claude-code.env")
		if _, err := os.Stat(envFile); err != nil {
			return "", fmt.Errorf("%s is not a localcode checkout: no %s", candidate, envFile)
		}
		return candidate, nil
	}
	return "", errors.New("no localcode checkout: install with `make install`, or pass -checkout")
}

// serverUp reports whether the endpoint can serve, rather than whether something is
// listening. llama-server answers 503 while it loads a model, and a session started then
// spends its first turn on an error instead of the work.
func serverUp(endpoint string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(endpoint + "/health")
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return fmt.Errorf("no answer from %s: start one with `make serve CONFIG=config/agent.env`", endpoint)
		}
		return fmt.Errorf("no server at %s: start one with `make serve CONFIG=config/agent.env`", endpoint)
	}
	defer resp.Body.Close() //nolint:errcheck // reading the code is the whole check
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server at %s is not ready: HTTP %d", endpoint, resp.StatusCode)
	}
	return nil
}

// status reports what the endpoint is actually serving, rather than that something is
// listening. The served context is the number worth printing: it is the one limit a
// session hits without warning, and it comes from the server rather than from a config
// file that may not be the one running.
func status(endpoint string) (int, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(endpoint + "/props")
	if err != nil {
		fmt.Printf("no server at %s\n", endpoint)
		return 1, nil
	}
	defer resp.Body.Close() //nolint:errcheck // the body is decoded below or the call failed
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("server at %s is not ready: HTTP %d\n", endpoint, resp.StatusCode)
		return 1, nil
	}
	var props struct {
		ModelPath string `json:"model_path"`
		Settings  struct {
			NCtx int `json:"n_ctx"`
		} `json:"default_generation_settings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&props); err != nil {
		return 2, fmt.Errorf("could not read %s/props: %w", endpoint, err)
	}
	fmt.Printf("serving %s at %d ctx on %s\n", filepath.Base(props.ModelPath), props.Settings.NCtx, endpoint)
	return 0, nil
}

// ensureServer starts one when nothing is serving, because requiring a second terminal is
// the headache this command exists to remove. What it costs is printed before it is spent
// rather than discovered afterwards: twenty seconds and most of the machine's memory are
// not something to find out about by waiting.
func ensureServer(root, endpoint, config string, noServe bool) error {
	if serverUp(endpoint) == nil {
		return nil
	}
	if noServe {
		return fmt.Errorf("no server at %s, and -no-serve was given", endpoint)
	}
	fmt.Fprintf(os.Stderr, "no server at %s\n", endpoint)
	fmt.Fprintf(os.Stderr, "starting %s — about 20s to load, ~17 GB resident while it runs\n", config)
	fmt.Fprintf(os.Stderr, "(a first run downloads ~17 GB and takes considerably longer)\n")

	logPath, err := serverLog()
	if err != nil {
		return err
	}
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("could not open %s: %w", logPath, err)
	}
	defer log.Close() //nolint:errcheck // the child holds its own descriptor

	cmd := exec.Command(filepath.Join(root, "scripts", "serve.sh"), config)
	// serve.sh resolves a relative chat-template path against its working directory, so
	// this has to be the checkout or the server loads the model's own template instead.
	cmd.Dir = root
	cmd.Stdout, cmd.Stderr = log, log
	// Its own session, so the terminal's Ctrl-C reaches the agent and not the server the
	// next session will want. It outlives this process deliberately; `localcode stop` ends it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start the server: %w", err)
	}
	if err := waitHealthy(endpoint, 20*time.Minute); err != nil {
		return fmt.Errorf("%w — see %s", err, logPath)
	}
	fmt.Fprintln(os.Stderr, "server ready")
	return nil
}

func waitHealthy(endpoint string, limit time.Duration) error {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if serverUp(endpoint) == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("the server did not become ready within %s", limit)
}

// serverLog is under the state directory rather than in the repository being worked in,
// for the same reason everything else here is.
func serverLog() (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "serve.log"), nil
}

func stateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("no home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "localcode"), nil
}

// script runs one of the checkout's own scripts and passes its exit code through. serve
// and stop are the same two `make serve` and `make stop` reach: the wait for the memory
// back is a fact with one home, and a launcher that re-solved it would be the fifth.
func script(checkoutFlag, name string, args ...string) (int, error) {
	root, err := resolveCheckout(checkoutFlag)
	if err != nil {
		return 2, err
	}
	cmd := exec.Command(filepath.Join(root, "scripts", name), args...)
	cmd.Dir = root // serve.sh resolves a relative chat-template path against its own $PWD
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), nil
		}
		return 2, fmt.Errorf("could not run %s: %w", name, err)
	}
	return 0, nil
}

// repoState is where one repository's session state lives — the handoff above all. Keyed
// by the repository's path rather than its name, since two checkouts of one project are
// the normal case here and they are not the same box of work.
func repoState(repo string) (string, error) {
	base, err := stateDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(repo))
	slug := filepath.Base(repo) + "-" + hex.EncodeToString(sum[:4])
	dir := filepath.Join(base, "repos", slug)
	return dir, os.MkdirAll(dir, 0o755)
}

// writeSettings renders the hooks with absolute paths. harness/claude-code/hooks.json
// addresses them through $CLAUDE_PROJECT_DIR, which is the repository being visited — so
// in anybody else's the hooks resolve to scripts that are not there and the session dies
// saying so. Generated rather than committed, because the path is only known once
// installed.
//
// The gate is this binary run as a hook rather than a fourth script. It reads a transcript
// and counts against a budget, which is the half of the repository `make check` covers.
//
// Stop is installed only for a session answering one instruction. It fires whenever the
// agent finishes responding, which in an interactive session is every time it hands the
// keyboard back — refusing there would refuse the conversation itself.
func writeSettings(root, dir string, oneShot bool) (string, error) {
	script := func(name string) any {
		return command(filepath.Join(root, "harness", "claude-code", "hooks", name))
	}
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not find this binary to install it as a hook: %w", err)
	}
	hooks := map[string]any{
		"SessionStart": script("session-start.sh"),
		"PreCompact":   script("pre-compact.sh"),
		"SessionEnd":   script("session-end.sh"),
		"PreToolUse":   command(shellQuote(self) + " hook gate"),
	}
	if oneShot {
		hooks["Stop"] = command(shellQuote(self) + " hook stop")
	}
	doc := map[string]any{"hooks": hooks}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", fmt.Errorf("could not write %s: %w", path, err)
	}
	return path, nil
}

// command is one hook entry. Every matcher is empty, so each fires for everything its
// event covers.
func command(line string) any {
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

// hook runs one of this binary's own session hooks. The harness spawns them, so they get
// nothing but stdin and the environment, and the session directory carries the rest.
//
// Exit 2 is the harness's contract for "refuse this, and give the model what stderr said".
// Everything else exits 0: a hook that cannot do its job must be able to stop a session
// overrunning and must not be able to stop it working.
func hook(name string) (int, error) {
	v, err := chain.Hook(name, os.Stdin, os.Getenv("LOCALCODE_HANDOFF_DIR"))
	switch {
	case errors.Is(err, chain.ErrNoSpec):
		return 0, nil
	case err != nil:
		return 0, err
	case v.Deny:
		fmt.Fprintln(os.Stderr, v.Reason)
		return 2, nil
	case v.Input != nil:
		// The harness reads a permitted call back off stdout, and runs the tool with what
		// it finds there.
		body, err := json.Marshal(map[string]any{"hookSpecificOutput": map[string]any{
			"hookEventName":      "PreToolUse",
			"permissionDecision": "allow",
			"updatedInput":       v.Input,
		}})
		if err != nil {
			return 0, err
		}
		fmt.Println(string(body))
	}
	return 0, nil
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

// newChain makes the directory one run of the tool keeps its sessions in, and names it.
// Sortable and typable, because the name is what `-resume` takes and what `localcode
// sessions` lists.
//
// The directory is what reserves the name: two invocations in the same second would
// otherwise share a chain, and the second would inherit a handoff written for the first.
func newChain(state string) (dir, id string, err error) {
	if err := os.MkdirAll(filepath.Join(state, "chains"), 0o755); err != nil {
		return "", "", err
	}
	base := time.Now().Format("20060102-150405")
	for n := 1; n <= 100; n++ {
		id = base
		if n > 1 {
			id = fmt.Sprintf("%s-%d", base, n)
		}
		dir = filepath.Join(state, "chains", id)
		switch err := os.Mkdir(dir, 0o755); {
		case err == nil:
			return dir, id, nil
		case !os.IsExist(err):
			return "", "", err
		}
	}
	return "", "", fmt.Errorf("could not name a new chain under %s", filepath.Join(state, "chains"))
}

// handoffBriefing is the protocol, and it travels in the system prompt because that is the
// channel the model trusts. The same words arriving through a tool result were refused as
// injection — correctly, which is why the gate itself only reports a state.
func handoffBriefing(l chain.Limits, path string) string {
	return fmt.Sprintf(
		"This session is budgeted. It may spend %d tool calls, at most %d of them in any one "+
			"turn, and its context may reach %d tokens of the %d it has; past the session's "+
			"bounds every call is refused except writing the handoff. A refusal is the budget, "+
			"not a fault to work around.\n"+
			"Write the handoff to %s with the Write tool before you stop, in this shape:\n"+
			"# Handoff\n"+
			"**Box:** what you were asked to do, in one line\n"+
			"**Files:** each path that matters, and why it does\n"+
			"**Tried:** what you did and what came of it — the results themselves, not the "+
			"commands that produced them\n"+
			"**Next:** the one thing to do next, or `none` when the instruction is finished\n"+
			"A fresh session inherits that file and nothing else. Keep it under 40 lines.\n"+
			"A handoff you were given is what the session before you established, not a "+
			"suggestion to check: start from its **Next** rather than re-deriving what it "+
			"already recorded.\n"+
			"Your room is small, so finish a few things rather than surveying many. `Edit` "+
			"refuses a file this session has not read, so a read you do not follow with a "+
			"change is room spent for nothing: read a file, change it, move to the next.",
		l.Calls, l.Batch, l.Ceiling, l.Window, path)
}

// sandboxExec is macOS's own. A var so a test can substitute a pass-through: the CI that
// runs `make check` is Linux, and the launcher's argument assembly is worth testing there
// even though the boundary itself can only be exercised on the machine VISION fixes.
var sandboxExec = "/usr/bin/sandbox-exec"

// writeSandboxProfile renders the policy: writes confined, reads open. Reads stay open
// because an agent that cannot read a toolchain cannot use one, and the risk that matters
// here is a mistaken write rather than a curious read.
//
// Every path is resolved first. On macOS /var, /tmp and /etc are symlinks into /private
// and seatbelt matches the resolved path, so an unresolved TMPDIR denies every compiler
// that uses one while appearing to allow it.
func writeSandboxProfile(state, cwd string, net bool) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("no home directory: %w", err)
	}
	writable := []string{
		cwd,   // the repository being worked in
		state, // this repository's handoff and settings
		os.TempDir(),
		"/private/tmp",
		filepath.Join(home, "Library", "Caches"), // where macOS toolchains cache
		filepath.Join(home, ".cache"),            // where XDG ones do
		filepath.Join(home, ".claude"),           // the agent's own history and project state
	}
	// Whatever this developer's ecosystems need, named once by them rather than guessed
	// once by us.
	writable = append(writable, extraWritable()...)

	var b strings.Builder
	b.WriteString("(version 1)\n(allow default)\n(deny file-write*)\n(allow file-write*\n")
	for _, p := range writable {
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			continue // absent is not an error: not every machine has every cache root
		}
		fmt.Fprintf(&b, "  (subpath %s)\n", sbplString(resolved))
	}
	// Writing to a terminal is not writing to the filesystem, and a shell needs these.
	b.WriteString("  (literal \"/dev/null\") (literal \"/dev/stdout\") (literal \"/dev/stderr\")\n")
	b.WriteString("  (literal \"/dev/dtracehelper\") (literal \"/dev/tty\"))\n")

	// A worktree beside the repository, and nothing else beside it. `kit claim` prints
	// `git worktree add ../<repo>-<id>`, which failed with `Operation not permitted`
	// because writes stopped at the working directory: the model recovered by putting the
	// worktree inside the repository, which works and is where nobody looks for it.
	//
	// Named after the repository rather than the parent opened up, because the parent is
	// where every other project of this developer's lives.
	if siblings := siblingWorktrees(cwd); siblings != "" {
		fmt.Fprintf(&b, "(allow file-write* (regex #\"%s\"))\n", siblings)
	}

	// Loopback reaches the model and nothing else reaches anywhere. It is what makes the
	// repository's source unable to leave the machine, and it is VISION's offline property
	// enforced rather than configured. -net is for the session that has to install
	// something, and it says so at startup rather than quietly.
	if !net {
		b.WriteString("(deny network*)\n")
		b.WriteString("(allow network-outbound (remote ip \"localhost:*\"))\n")
		b.WriteString("(allow network-inbound (local ip \"localhost:*\"))\n")
		b.WriteString("(allow network* (remote unix-socket))\n")
	}

	path := filepath.Join(state, "sandbox.sb")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", fmt.Errorf("could not write %s: %w", path, err)
	}
	return path, nil
}

// siblingWorktrees is the pattern matching directories beside the repository and named
// after it — `repo-0001` next to `repo` — and "" when there is no such place to name.
//
// Resolved first, for the reason every other path here is: seatbelt matches the resolved
// path, and a pattern built from an unresolved one denies what it appears to allow.
func siblingWorktrees(cwd string) string {
	resolved, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return ""
	}
	parent, base := filepath.Dir(resolved), filepath.Base(resolved)
	if parent == resolved || parent == "/" || base == "" {
		return ""
	}
	return "^" + sbplRegex(filepath.Join(parent, base)) + "-[^/]+"
}

// sbplRegex escapes a literal path for use inside a seatbelt regex. The path comes from
// the filesystem rather than from a person, and a repository called `foo.bar` would
// otherwise match `fooxbar` — and a `+` or a `(` would change the pattern outright.
func sbplRegex(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(`\.+*?()[]{}^$|`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return sbplEscape(b.String())
}

// sbplEscape makes a string safe inside the profile's double quotes, as sbplString does
// for a path.
func sbplEscape(s string) string {
	return strings.NewReplacer(`"`, `\"`).Replace(s)
}

// sbplString quotes a path for the profile. A path is attacker-adjacent here only in the
// sense that it comes from the filesystem, but an unescaped quote would end the string and
// change the policy, which is the one bug a sandbox must not have.
func sbplString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`
}

// writableConfigPath is the one place a developer widens the policy. It is a list of
// paths rather than a language: the tool learns no ecosystem, and the repository that
// needs ~/.cargo says so once.
func writableConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("no home directory: %w", err)
	}
	return filepath.Join(home, ".config", "localcode", "writable"), nil
}

func extraWritable() []string {
	path, err := writableConfigPath()
	if err != nil {
		return nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil // absent is the normal case
	}
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				line = filepath.Join(home, line[2:])
			}
		}
		out = append(out, line)
	}
	return out
}

// worktreeBriefing names where a git worktree may go, because the session cannot find out
// except by being refused: `kit claim` prints `git worktree add ../<repo>-<id>`, and a
// session that met `Operation not permitted` there put the worktree somewhere nobody looks
// for it. `.worktrees/` is named first where the repository already keeps one, since a
// repository with that directory has decided where they go.
func worktreeBriefing(cwd string) string {
	where := "beside it, named after it — `../" + filepath.Base(cwd) + "-<id>`"
	if info, err := os.Stat(filepath.Join(cwd, ".worktrees")); err == nil && info.IsDir() {
		where = "`.worktrees/<id>` inside it, or " + where
	}
	return "A git worktree may go in " + where + ", and nowhere else. "
}

// sandboxBriefing tells the session what it is inside, so a refusal comes back as an
// explanation with a fix rather than as a puzzle. Kept to a few lines: it is paid for on
// every request in a context this small.
func sandboxBriefing(cwd string) string {
	cfg, err := writableConfigPath()
	if err != nil {
		cfg = "~/.config/localcode/writable"
	}
	return "You are running in a sandbox that confines writes to " + cwd +
		", temp directories and cache roots. Reading anywhere is allowed. " +
		worktreeBriefing(cwd) +
		"If a command fails with `operation not permitted` on a path outside those, " +
		"do not work around it: report the path, and tell the user it is allowed by " +
		"adding that path to " + cfg + "."
}
