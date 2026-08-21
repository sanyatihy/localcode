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
//	0  the agent ran and exited cleanly, or the subcommand answered yes
//	1  the agent ran and exited non-zero, or the subcommand answered no
//	2  the command could not be carried out (no checkout, no server, bad flags)
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
	"strings"
	"syscall"
	"time"

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
  localcode --help

flags:
  -checkout dir   the localcode checkout to read configuration from
  -endpoint url   the server to use
  -config file    the serving config to start (default config/agent.env)
  -no-serve       refuse if no server is running, rather than starting one
`

func main() {
	fs := flag.NewFlagSet("localcode", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	checkoutFlag := fs.String("checkout", "", "the localcode checkout to read configuration from")
	endpoint := fs.String("endpoint", "http://127.0.0.1:8081", "the server to use")
	config := fs.String("config", "config/agent.env", "the serving config to start")
	noServe := fs.Bool("no-serve", false, "refuse if no server is running")
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
	default:
		code, err = run(opts{
			checkout: *checkoutFlag,
			endpoint: *endpoint,
			config:   *config,
			noServe:  *noServe,
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

	// The handoff hooks are addressed absolutely and given a state directory of their
	// own, which is what lets 0016 run outside this checkout at all.
	state, err := repoState(cwd)
	if err != nil {
		return 2, err
	}
	settings, err := writeSettings(root, state)
	if err != nil {
		return 2, err
	}
	env = append(env, "LOCALCODE_HANDOFF_DIR="+state)

	argv := []string{"--tools", agentTools, "--allowedTools", agentTools, "--settings", settings}
	// The instruction goes last and only when there is one: with no prompt this is an
	// interactive session, which is the common case for a developer in their own repo.
	if len(o.args) > 0 {
		argv = append(argv, "-p")
		argv = append(argv, o.args...)
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
	profile, err := writeSandboxProfile(state, cwd)
	if err != nil {
		return 2, err
	}
	sandboxArgv := append([]string{"-f", profile, claudePath}, argv...)

	cmd := exec.Command(sandboxExec, sandboxArgv...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), nil
		}
		return 2, fmt.Errorf("could not start claude: %w", err)
	}
	return 0, nil
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
func writeSettings(root, state string) (string, error) {
	hook := func(name string) any {
		return []any{map[string]any{"hooks": []any{map[string]string{
			"type":    "command",
			"command": filepath.Join(root, "harness", "claude-code", "hooks", name),
		}}}}
	}
	doc := map[string]any{"hooks": map[string]any{
		"SessionStart": hook("session-start.sh"),
		"PreCompact":   hook("pre-compact.sh"),
		"SessionEnd":   hook("session-end.sh"),
	}}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(state, "settings.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", fmt.Errorf("could not write %s: %w", path, err)
	}
	return path, nil
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
func writeSandboxProfile(state, cwd string) (string, error) {
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

	path := filepath.Join(state, "sandbox.sb")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", fmt.Errorf("could not write %s: %w", path, err)
	}
	return path, nil
}

// sbplString quotes a path for the profile. A path is attacker-adjacent here only in the
// sense that it comes from the filesystem, but an unescaped quote would end the string and
// change the policy, which is the one bug a sandbox must not have.
func sbplString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`
}
