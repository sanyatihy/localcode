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
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

const usage = `localcode — drive the local model in this repository

usage:
  localcode [flags] [prompt]   run the agent here
  localcode serve              start the server here, in the foreground
  localcode stop               stop it, waiting for the memory back
  localcode status             what is being served, if anything
  localcode sessions           the chains this repository has run
  localcode account [id]       what one chain cost: minutes, tokens, its rates
  localcode hook <name>        run one of this binary's own session hooks
  localcode --help

flags:
  -checkout dir      the localcode checkout to read configuration from
  -harness name      which agent to run the sessions in (%s)
  -endpoint url      the server to use
  -config file       the serving config to start (default config/agent.env)
  -no-serve          refuse if no server is running, rather than starting one
  -net               allow outbound network for this session (loopback only by default)
  -ceiling pct       lower the ceiling below what the reserve already allows
  -calls n           override the tool-call budget the ceiling implies
  -sessions n        how many sessions one instruction may take
  -session-timeout d how long one session may run before it is stopped
  -continue          carry on this repository's most recent chain
  -resume id         carry on the chain with this id
  -fork id           start a chain from what that chain knew
  -jsonl file        append the account subcommand's report to this results file too
`

func main() {
	fs := flag.NewFlagSet("localcode", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprintf(os.Stderr, usage, harness.Names()) }
	checkoutFlag := fs.String("checkout", "", "the localcode checkout to read configuration from")
	agentName := fs.String("harness", harness.DefaultAgent, "which agent to run the sessions in")
	endpoint := fs.String("endpoint", "http://127.0.0.1:8081", "the server to use")
	config := fs.String("config", "config/agent.env", "the serving config to start")
	noServe := fs.Bool("no-serve", false, "refuse if no server is running")
	net := fs.Bool("net", false, "allow outbound network for this session")
	ceiling := fs.Int("ceiling", 100, "how much of the window a session may fill, in percent")
	calls := fs.Int("calls", 0, "override the tool-call budget the ceiling implies")
	// Both bounds are backstops rather than budgets: what should end a session is its
	// ceiling and what should end a chain is the work being done. Sized so that neither
	// binds first on real source, where a session runs 15 to 30 minutes and a chain of one
	// instruction has taken 36 of them.
	sessions := fs.Int("sessions", 60, "how many sessions one instruction may take")
	timeout := fs.Duration("session-timeout", time.Hour, "how long one session may run")
	cont := fs.Bool("continue", false, "carry on this repository's most recent chain")
	resume := fs.String("resume", "", "carry on the chain with this id")
	fork := fs.String("fork", "", "start a chain from what the chain with this id knew")
	jsonl := fs.String("jsonl", "", "append what account reports to this results file as well")
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
	case len(args) > 0 && args[0] == "account":
		code, err = accountHere(os.Stdout, strings.Join(args[1:], ""), *jsonl)
	default:
		code, err = run(opts{
			harness:  *agentName,
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
	harness  string
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
	// Before the server, because a bound below one runs no session at all: the loop skips
	// its body and records a chain that stopped at its bound one session before it began.
	// Finding that out by starting a model costs twenty seconds and most of the machine.
	if o.sessions < 1 {
		return 2, fmt.Errorf("-sessions is %d: a bound below one runs nothing", o.sessions)
	}
	root, err := resolveCheckout(o.checkout)
	if err != nil {
		return 2, err
	}
	if err := ensureServer(root, o.endpoint, o.config, o.noServe); err != nil {
		return 2, err
	}

	// The agent runs the sessions, and everything that differs between agents is behind
	// it: the flags, the environment, the window it is told it has, where it files what a
	// session cost. Built before the work so a harness that cannot be configured is
	// refused rather than discovered on the first session.
	name := o.harness
	if name == "" {
		name = harness.DefaultAgent
	}
	agent, err := harness.NewAgent(name, root)
	if err != nil {
		return 2, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return 2, fmt.Errorf("no working directory: %w", err)
	}

	// The budget is derived from the window the harness is told it has, so a served config
	// and the enforcement over it cannot disagree. Refused rather than guessed: a session
	// started in a window nothing fits in spends a cold ingest to say `Prompt is too long`.
	//
	// What is served is read off the server rather than off any file, because a file
	// carries one number and `-config` chooses which context is served. What the harness
	// makes of it is the harness's: the reservation it keeps for a reply is its own.
	served, err := servedContext(o.endpoint)
	if err != nil {
		return 2, err
	}
	maxContext, maxOutput, err := agent.Window(served)
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
	if err := agent.Prepare(chainDir); err != nil {
		return 2, err
	}

	// The agent runs inside a seatbelt sandbox, which is what makes exposing an
	// unrestricted Bash tool defensible: the boundary is the kernel's rather than the
	// model's judgement, and it needs to know nothing about the language in the repository.
	//
	// Refused rather than skipped: running unsandboxed because the sandbox is missing is
	// the one failure mode that would be silent and would matter.
	if _, err := os.Stat(sandboxExec); err != nil {
		return 2, fmt.Errorf("no %s: localcode runs the agent sandboxed and will not run it otherwise", sandboxExec)
	}
	if o.net {
		fmt.Fprintln(os.Stderr, "network: outbound ENABLED for this session")
	}
	profile, err := writeSandboxProfile(state, cwd, o.net, agent.Writable())
	if err != nil {
		return 2, err
	}

	l := launch{
		agent: agent, sandbox: sandboxExec, profile: profile,
		limits: limits, briefing: sandboxBriefing(cwd), cwd: cwd,
		endpoint: o.endpoint,
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
	// No channel: a session at a keyboard is in the terminal's own foreground group, which is
	// what delivers an interrupt to it, and there is no supervisor loop here to forward one.
	r, err := l.session(dir, n, id, "", chain.LatestHandoff(chainDir), nil, 0)
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
	return listChains(state, os.Stdout)
}

// resolveCheckout prefers the flag, then the stamped path. It refuses rather than
// searching: the error names the one command that fixes it, which a guess cannot.
func resolveCheckout(flagValue string) (string, error) {
	for _, candidate := range []string{flagValue, checkout} {
		if candidate == "" {
			continue
		}
		// The server script rather than any harness's configuration: which agent a chain
		// runs in is a choice, and a checkout is a checkout before that choice is made.
		marker := filepath.Join(candidate, "scripts", "serve.sh")
		if _, err := os.Stat(marker); err != nil {
			return "", fmt.Errorf("%s is not a localcode checkout: no %s", candidate, marker)
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

// props is what the endpoint says it is serving. The served context is the number that
// matters: it is the one limit a session hits without warning, and it is a property of the
// running server rather than of a config file that may not be the one that started it.
type props struct {
	ModelPath string `json:"model_path"`
	Settings  struct {
		NCtx int `json:"n_ctx"`
	} `json:"default_generation_settings"`
}

// readProps asks the endpoint. A server that is not there is `nil, nil`: `status` answers
// that as "no", and `run` never sees it because `ensureServer` has already been past.
func readProps(endpoint string) (*props, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(endpoint + "/props")
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close() //nolint:errcheck // the body is decoded below or the call failed
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: server at %s answered HTTP %d", errNotReady, endpoint, resp.StatusCode)
	}
	var p props
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, fmt.Errorf("could not read %s/props: %w", endpoint, err)
	}
	return &p, nil
}

// servedContext is what the endpoint says it is serving, which is the only number a
// session can be budgeted against. The wall a session hits is the served context, and
// `-config` moves it, so a context taken from a file is a claim about whichever server
// that file was written for. Taking a file at face value is what killed the sessions the
// enforcement was first measured on: a budget 3,072 tokens too generous let them edit four
// files each and then die on `Prompt is too long` with no handoff written.
func servedContext(endpoint string) (int, error) {
	p, err := readProps(endpoint)
	if err != nil {
		return 0, err
	}
	if p == nil || p.Settings.NCtx <= 0 {
		return 0, fmt.Errorf("%s serves no context it will report, so a session cannot be "+
			"budgeted against it: what a session may spend is derived from what is served",
			endpoint)
	}
	return p.Settings.NCtx, nil
}

// errNotReady separates a server that is still loading from one whose answer could not be
// read: the first is a state to report and wait out, the second is a command that could
// not be carried out.
var errNotReady = errors.New("not ready")

// status reports what the endpoint is actually serving, rather than that something is
// listening.
func status(endpoint string) (int, error) {
	p, err := readProps(endpoint)
	switch {
	case errors.Is(err, errNotReady):
		fmt.Println(err)
		return 1, nil
	case err != nil:
		return 2, err
	case p == nil:
		fmt.Printf("no server at %s\n", endpoint)
		return 1, nil
	}
	fmt.Printf("serving %s at %d ctx on %s\n", filepath.Base(p.ModelPath), p.Settings.NCtx, endpoint)
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
	// serve.sh validates the config and then execs the server, so this process is the
	// server and its death is the config being refused. Watched rather than waited out: the
	// default config names a build that is not the binary on PATH, and a machine without it
	// would otherwise poll a health endpoint for twenty minutes before saying so.
	died := make(chan error, 1)
	go func() { died <- cmd.Wait() }()
	if err := waitHealthy(endpoint, 20*time.Minute, died); err != nil {
		return fmt.Errorf("%w — see %s", err, logPath)
	}
	fmt.Fprintln(os.Stderr, "server ready")
	return nil
}

func waitHealthy(endpoint string, limit time.Duration, died <-chan error) error {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if serverUp(endpoint) == nil {
			return nil
		}
		select {
		case err := <-died:
			return fmt.Errorf("the server exited before it was ready: %w", err)
		case <-time.After(2 * time.Second):
		}
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

// credentialRoots are the directories a session may not read, relative to the home
// directory. Each holds credentials and nothing else, which is what makes denying it safe:
// a directory a build also reads configuration from is not a candidate.
var credentialRoots = []string{".ssh", ".aws", ".gnupg", ".config/gh", "Library/Keychains"}

// deniedRead resolves the credential roots against home. Home is resolved once and the
// roots built from it, rather than each root resolved in turn: a root this machine has not
// created yet still has to be denied for the day it is.
func deniedRead(home string) []string {
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved
	}
	out := make([]string, 0, len(credentialRoots))
	for _, r := range credentialRoots {
		out = append(out, filepath.Join(home, filepath.FromSlash(r)))
	}
	return out
}

// writeSandboxProfile renders the policy: writes confined, reads open but for the
// credential roots. Reads stay open because an agent that cannot read a toolchain cannot
// use one; the roots are the exception, because the working tree is a channel off this
// machine — a human pushes it — and reading a key is the first half of sending it.
//
// Every path is resolved first. On macOS /var, /tmp and /etc are symlinks into /private
// and seatbelt matches the resolved path, so an unresolved TMPDIR denies every compiler
// that uses one while appearing to allow it.
func writeSandboxProfile(state, cwd string, net bool, agentState []string) (string, error) {
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
	}
	// Where the agent files its own state, which only the agent knows.
	writable = append(writable, agentState...)
	// Whatever this developer's ecosystems need, named once by them rather than guessed
	// once by us.
	writable = append(writable, extraWritable(os.Stderr)...)

	var b strings.Builder
	b.WriteString("(version 1)\n(allow default)\n")
	// After the allow, because the last matching rule is the one seatbelt applies.
	b.WriteString("(deny file-read*\n")
	for _, p := range deniedRead(home) {
		fmt.Fprintf(&b, "  (subpath %s)\n", sbplString(p))
	}
	b.WriteString(")\n")
	b.WriteString("(deny file-write*)\n(allow file-write*\n")
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

	// A worktree beside the repository, and nothing else beside it. `git worktree add
	// ../<repo>-<name>` is the common form and it failed with `Operation not permitted`,
	// because writes stopped at the working directory: the session recovered by putting the
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

// extraWritable reads that config and says what it opened. A line reading `/` widens the
// policy further than -net does and -net is the one that announces itself, so each path
// this file opens is named on the way past.
//
// A line covering a credential root is refused instead. Denying the read while allowing
// the write leaves the deny-list advisory: moving ~/.ssh/id_rsa into the repository needs
// no read at all.
func extraWritable(w io.Writer) []string {
	path, err := writableConfigPath()
	if err != nil {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	// Resolved, so a `~/.ssh` here and the denied root compare as the same path.
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil // absent is the normal case
	}
	denied := deniedRead(home)
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "~/") {
			line = filepath.Join(home, line[2:])
		}
		line = filepath.Clean(line)
		if root := coversDeniedRoot(line, denied); root != "" {
			_, _ = fmt.Fprintf(w, "writable: refused %s — it opens %s, which holds credentials\n", line, root)
			continue
		}
		out = append(out, line)
	}
	if len(out) > 0 {
		_, _ = fmt.Fprintf(w, "writable: %s opened %s\n", path, strings.Join(out, ", "))
	}
	return out
}

// coversDeniedRoot names the credential root a writable path would reach, and "" when it
// reaches none. Either direction counts: a path inside a root and a path above one both
// hand the session the files the deny-list took away.
//
// Symlinks are resolved first where the path exists, because a link into a credential root
// is the same widening spelled differently.
func coversDeniedRoot(path string, denied []string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	for _, root := range denied {
		if within(path, root) || within(root, path) {
			return root
		}
	}
	return ""
}

// within reports whether child is parent or sits under it.
func within(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// worktreeBriefing names where a git worktree may go, because the session cannot find out
// except by being refused, and the one that was refused put it somewhere nobody looks for
// it. `.worktrees/` is named first where the repository already keeps one, since a
// repository with that directory has decided where they go.
//
// The rule is the repository's own name, not any tool's convention. That it matches what
// `kit claim` prints is why the gap was found and not what the policy is for.
func worktreeBriefing(cwd string) string {
	where := "beside it, named after it — `../" + filepath.Base(cwd) + "-<name>`"
	if info, err := os.Stat(filepath.Join(cwd, ".worktrees")); err == nil && info.IsDir() {
		where = "`.worktrees/<name>` inside it, or " + where
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
	// Named in the tilde form, which is shorter than five absolute paths in a context
	// this small and is how a developer writes them back.
	denied := make([]string, 0, len(credentialRoots))
	for _, r := range credentialRoots {
		denied = append(denied, "~/"+r)
	}
	return "You are running in a sandbox that confines writes to " + cwd +
		", temp directories and cache roots. Reading is allowed everywhere except " +
		strings.Join(denied, ", ") + ", which hold credentials this work does not need: " +
		"a refusal there is final, so report it rather than routing around it. " +
		worktreeBriefing(cwd) +
		"If a write fails with `operation not permitted` on a path outside those, " +
		"do not work around it: report the path, and tell the user it is allowed by " +
		"adding that path to " + cfg + "."
}
