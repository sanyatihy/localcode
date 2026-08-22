package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/sanyatihy/localcode/internal/chain"
	"github.com/sanyatihy/localcode/internal/eval"
)

// launch is everything a session needs that does not change from one to the next. Built
// once, so a chain cannot serve its third session a different configuration from its first.
type launch struct {
	claude   string
	sandbox  string
	profile  string
	settings string
	env      []string
	limits   chain.Limits
	briefing string
}

// row is one session of a chain. The file of them is what a chain can be read back from
// when its last handoff does not explain how it got there.
type row struct {
	At      string `json:"at"`
	Chain   string `json:"chain"`
	Session int    `json:"session"`
	Seconds int    `json:"seconds"`
	Exit    int    `json:"exit"`
	Peak    int    `json:"peak_context_tokens"`
	Turns   int    `json:"turns"`
	Calls   int    `json:"tool_calls"`
	Handoff int    `json:"handoff_bytes"`
	Next    string `json:"next"`
}

// session runs one, in its own directory, and returns what the harness exited with.
//
// The directory is fresh every time: a handoff left in it by the session before is a stale
// instruction the next one obeys, which is how a chain freezes at its first handoff.
func (l launch) session(dir string, n int, chainID, goal, inherit string) (row, error) {
	handoffPath := filepath.Join(dir, chain.HandoffName)
	if err := chain.WriteSpec(dir, chain.Spec{
		Limits: l.limits, Handoff: handoffPath, Chain: chainID, Session: n,
	}); err != nil {
		return row{}, err
	}

	argv := []string{
		"--tools", agentTools, "--allowedTools", agentTools,
		"--settings", l.settings,
		// The handoff is written outside the repository being visited, so a session leaves
		// it with exactly the files the work changed. Claude Code confines its file tools
		// to the workspace, and this is what puts that one directory in it.
		"--add-dir", dir,
		"--append-system-prompt", l.briefing + "\n\n" + handoffBriefing(l.limits, handoffPath),
	}
	// The instruction goes last and verbatim. What the session before it learned arrives
	// separately, through the hook 0016 already uses, so a chain cannot drift by rewriting
	// its own goal at each hop.
	if goal != "" {
		argv = append(argv, "-p", goal)
	}

	env := append(append([]string{}, l.env...), "LOCALCODE_HANDOFF_DIR="+dir)
	if inherit != "" {
		env = append(env, "LOCALCODE_INHERIT="+inherit)
	}

	cmd := exec.Command(l.sandbox, append([]string{"-f", l.profile, l.claude}, argv...)...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	started := time.Now()
	err := cmd.Run()
	r := row{
		At: started.UTC().Format(time.RFC3339), Chain: chainID, Session: n,
		Seconds: int(time.Since(started).Round(time.Second).Seconds()),
	}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		r.Exit = exit.ExitCode()
	default:
		return r, fmt.Errorf("could not start claude: %w", err)
	}

	body := chain.Read(handoffPath)
	r.Handoff, r.Next = len(body), chain.Next(body)
	if t := newestTranscript(dir, env); t != "" {
		r.Peak, r.Turns = chain.Cost(t)
	}
	r.Calls = chain.Counter(dir, chain.CallsFile(sessionIDOf(dir)))
	return r, nil
}

// runChain runs sessions until the instruction is finished, the chain stops making
// progress, or the bound is reached. Each of the three is a different answer and each
// says so: a chain that stopped for its bound has work left, and one that stalled has a
// handoff to read.
func runChain(l launch, chainDir, chainID, goal string, bound int) (int, error) {
	inherit := chain.LatestHandoff(chainDir)
	first := chain.NextSession(chainDir)
	var prev string
	var havePrev bool
	if inherit != "" {
		prev, havePrev = chain.Next(chain.Read(inherit)), true
	}

	// A chain is one thing to interrupt. The session shares this process group, so Ctrl-C
	// reaches it too — and without this the supervisor reads that death as a finished
	// session and starts the next one.
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt)
	defer signal.Stop(interrupted)

	for n := first; n < first+bound; n++ {
		dir := filepath.Join(chainDir, fmt.Sprintf("%02d", n))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 2, err
		}
		r, err := l.session(dir, n, chainID, goal, inherit)
		if err != nil {
			return 2, err
		}
		if err := eval.AppendJSON(filepath.Join(chainDir, "sessions.jsonl"), r); err != nil {
			return 2, err
		}
		body := chain.Read(filepath.Join(dir, chain.HandoffName))
		fmt.Fprintf(os.Stderr, "session %d — %d tool calls, %d of %d tokens, %ds — next: %s\n",
			n, r.Calls, r.Peak, l.limits.Window, r.Seconds, or(r.Next, "nothing recorded"))

		if chain.Done(body) {
			fmt.Fprintf(os.Stderr, "chain %s finished after %s\n", chainID, plural(n-first+1, "session"))
			return 0, nil
		}
		// Two sessions planning the same next step is the shape a chain fails in: it is
		// still writing handoffs, and none of them is progress.
		if havePrev && r.Next == prev {
			fmt.Fprintf(os.Stderr, "chain %s stopped: this session planned what the last one "+
				"did — read %s\n", chainID, filepath.Join(dir, chain.HandoffName))
			return 1, nil
		}
		select {
		case <-interrupted:
			fmt.Fprintf(os.Stderr, "chain %s interrupted — continue with `localcode -resume %s`\n",
				chainID, chainID)
			return 1, nil
		default:
		}
		prev, havePrev = r.Next, true
		inherit = filepath.Join(dir, chain.HandoffName)
	}
	fmt.Fprintf(os.Stderr, "chain %s stopped after %s, which is its bound — "+
		"read %s and continue with `localcode -resume %s`\n",
		chainID, plural(bound, "session"), chain.LatestHandoff(chainDir), chainID)
	return 1, nil
}

func plural(n int, thing string) string {
	if n == 1 {
		return "1 " + thing
	}
	return fmt.Sprintf("%d %ss", n, thing)
}

func or(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// sessionIDOf reads back the session id the gate counted against. The counter is named
// after it because a directory can outlive the session that filled it, and a count carried
// into the next one would spend a budget nobody used. A session that called no tool leaves
// no counter, and nothing here reports what it cost.
func sessionIDOf(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if id, ok := strings.CutPrefix(e.Name(), "calls-"); ok {
			return id
		}
	}
	return ""
}

// newestTranscript finds what the session wrote about itself. Claude Code files a
// transcript under its config directory, keyed by the working directory, so this asks the
// environment the session ran with rather than guessing where that is.
func newestTranscript(dir string, env []string) string {
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
	id := sessionIDOf(dir)
	if id == "" {
		return ""
	}
	matches, err := filepath.Glob(filepath.Join(config, "projects", "*", id+".jsonl"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	return matches[0]
}

// listChains prints what this repository has been asked to do, newest first. It lists
// chains rather than sessions because a chain is what `-resume` takes: the sessions inside
// one are a detail of how far it got.
func listChains(state string) (int, error) {
	ids, err := chain.Chains(state)
	if err != nil {
		return 2, err
	}
	if len(ids) == 0 {
		fmt.Println("no chains here yet")
		return 1, nil
	}
	for _, id := range ids {
		dir := filepath.Join(state, "chains", id)
		body := chain.Read(chain.LatestHandoff(dir))
		state := or(chain.Next(body), "nothing recorded")
		if chain.Done(body) {
			state = "finished"
		}
		fmt.Printf("%s  %s  %s\n", id, plural(chain.NextSession(dir)-1, "session"), state)
	}
	return 0, nil
}

// selectChain answers which chain this invocation belongs to, and what it was asked to do.
// Starting clean is the default because that is what `claude` does, and because one handoff
// per repository was wrong: a second instruction in the same checkout would have resumed
// the first and then overwritten what it knew.
func selectChain(state string, o opts) (dir, id, goal string, err error) {
	goal = strings.TrimSpace(strings.Join(o.args, " "))
	switch {
	case o.resume != "":
		dir, id = filepath.Join(state, "chains", o.resume), o.resume
		if _, err := os.Stat(dir); err != nil {
			return "", "", "", fmt.Errorf("no chain %s here: `localcode sessions` lists them", o.resume)
		}
	case o.cont:
		ids, err := chain.Chains(state)
		if err != nil {
			return "", "", "", err
		}
		if len(ids) == 0 {
			return "", "", "", errors.New("nothing to continue here: this repository has no chains yet")
		}
		id = ids[0]
		dir = filepath.Join(state, "chains", id)
	case o.fork != "":
		from := filepath.Join(state, "chains", o.fork)
		if _, err := os.Stat(from); err != nil {
			return "", "", "", fmt.Errorf("no chain %s here: `localcode sessions` lists them", o.fork)
		}
		dir, id, err = newChain(state)
		if err != nil {
			return "", "", "", err
		}
		if err := forkFrom(from, dir); err != nil {
			return "", "", "", err
		}
	default:
		if dir, id, err = newChain(state); err != nil {
			return "", "", "", err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", "", err
	}

	// An instruction given now replaces the one the chain was started with; given none, the
	// chain re-issues its own. That is what stops a goal drifting through a chain, and it
	// is also what makes `-continue` with a new instruction mean what it looks like.
	path := filepath.Join(dir, "goal.txt")
	if goal == "" {
		goal = strings.TrimSpace(string(chain.Read(path)))
	} else if err := os.WriteFile(path, []byte(goal), 0o644); err != nil {
		return "", "", "", err
	}
	return dir, id, goal, nil
}

// forkFrom seeds a new chain with what another one had learned, and with nothing else. A
// fork is a second attempt from the same knowledge, so it takes the handoff and the goal
// and leaves the sessions behind.
func forkFrom(from, to string) error {
	if err := os.MkdirAll(filepath.Join(to, "00"), 0o755); err != nil {
		return err
	}
	if body := chain.Read(chain.LatestHandoff(from)); body != nil {
		if err := os.WriteFile(filepath.Join(to, "00", chain.HandoffName), body, 0o644); err != nil {
			return err
		}
	}
	if goal := chain.Read(filepath.Join(from, "goal.txt")); goal != nil {
		return os.WriteFile(filepath.Join(to, "goal.txt"), goal, 0o644)
	}
	return nil
}
