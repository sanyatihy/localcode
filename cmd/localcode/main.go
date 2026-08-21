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
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
  localcode status             what is being served, if anything
  localcode --help

flags:
  -checkout dir   the localcode checkout to read configuration from
  -endpoint url   the server to use
`

func main() {
	fs := flag.NewFlagSet("localcode", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	checkoutFlag := fs.String("checkout", "", "the localcode checkout to read configuration from")
	endpoint := fs.String("endpoint", "http://127.0.0.1:8081", "the server to use")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	args := fs.Args()
	var (
		code int
		err  error
	)
	if len(args) > 0 && args[0] == "status" {
		code, err = status(*endpoint)
	} else {
		code, err = run(*checkoutFlag, *endpoint, args)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "localcode: "+err.Error())
	}
	os.Exit(code)
}

func run(checkoutFlag, endpoint string, args []string) (int, error) {
	root, err := resolveCheckout(checkoutFlag)
	if err != nil {
		return 2, err
	}
	if err := serverUp(endpoint); err != nil {
		return 2, err
	}

	env, err := harness.EnvFromFile(filepath.Join(root, "harness", "claude-code", "claude-code.env"))
	if err != nil {
		return 2, err
	}

	argv := []string{"--tools", agentTools, "--allowedTools", agentTools}
	// The instruction goes last and only when there is one: with no prompt this is an
	// interactive session, which is the common case for a developer in their own repo.
	if len(args) > 0 {
		argv = append(argv, "-p")
		argv = append(argv, args...)
	}

	cmd := exec.Command("claude", argv...)
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
