package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sanyatihy/localcode/internal/chain"
	"github.com/sanyatihy/localcode/internal/eval"
)

// launch is everything a session needs that does not change from one to the next. Built
// once, so a chain cannot serve its third session a different configuration from its first.
// progressOut is where the supervisor's own lines go. A var so a test can silence them:
// a chain under test narrates every session it runs, and thirty of those buried the one
// real failure in a CI log.
var progressOut io.Writer = os.Stderr

// narrate writes one of the supervisor's own lines. A terminal that has gone away is not
// something a chain can act on, and stopping a run to report it would end the work.
func narrate(format string, args ...any) {
	_, _ = fmt.Fprintf(progressOut, format, args...)
}

type launch struct {
	claude   string
	sandbox  string
	profile  string
	settings string
	env      []string
	limits   chain.Limits
	briefing string
	timeout  time.Duration
	cwd      string
}

// row is one session of a chain. The file of them is what a chain can be read back from
// when its last handoff does not explain how it got there.
type row struct {
	At       string `json:"at"`
	Chain    string `json:"chain"`
	Session  int    `json:"session"`
	Seconds  int    `json:"seconds"`
	Exit     int    `json:"exit"`
	Peak     int    `json:"peak_context_tokens"`
	Turns    int    `json:"turns"`
	Calls    int    `json:"tool_calls"`
	Handoff  int    `json:"handoff_bytes"`
	Next     string `json:"next"`
	TimedOut bool   `json:"timed_out,omitempty"`
	// Whether the repository moved while the session ran, and absent when there was no
	// repository to read. Absent is not `false`: a chain outside version control has not
	// stalled, it has nothing here to be judged by.
	Moved *bool `json:"repo_moved,omitempty"`
	// Whether an interrupt reached this session. Recorded beside TimedOut and for the same
	// reason: a chain read back afterwards cannot tell a session somebody stopped from one
	// that ended on its own.
	Interrupted bool `json:"interrupted,omitempty"`
}

// session runs one, in its own directory, and returns what the harness exited with.
//
// The directory is fresh every time: a handoff left in it by the session before is a stale
// instruction the next one obeys, which is how a chain freezes at its first handoff.
func (l launch) session(dir string, n int, chainID, goal, inherit string,
	interrupted <-chan os.Signal) (row, error) {
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
		// Events rather than a result, because a result arrives once and this machine takes
		// minutes to reach it. The supervisor renders them, so it owns every line printed.
		argv = append(argv, "--output-format", "stream-json", "--verbose",
			"--include-partial-messages", "-p", goal)
	}

	env := append(append([]string{}, l.env...), "LOCALCODE_HANDOFF_DIR="+dir)
	if inherit != "" {
		env = append(env, "LOCALCODE_INHERIT="+inherit)
	}

	// A wall-clock bound, because none of the others is one. Denied calls still cost a turn
	// apiece, and a turn at this depth is minutes: a session that answers a spent budget by
	// trying another tool rather than by handing off would otherwise run until the budget
	// of calls ran out, an hour later.
	// Read before the session starts, so what it is compared with is the repository as the
	// session was handed it rather than as the one before it left it.
	before := chain.Repo(l.cwd)

	ctx := context.Background()
	if l.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, l.timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, l.sandbox, append([]string{"-f", l.profile, l.claude}, argv...)...)
	cmd.Env = env
	cmd.Stderr = os.Stderr

	started := time.Now()
	var r row
	var err error
	if goal == "" {
		// A developer at a keyboard: the harness draws its own screen and owns the terminal.
		cmd.Stdin, cmd.Stdout = os.Stdin, os.Stdout
		err = cmd.Run()
	} else {
		// Nobody is typing, and a terminal that never closes makes the harness wait on
		// stdin it will not get.
		cmd.Stdin = nil
		// Its own process group, because the supervisor's is not one it can rely on. Started
		// in the background and signalled by pid, the supervisor takes the interrupt alone
		// and the session it was waiting on runs on against the endpoint — measured, an
		// orphan had to be matched by its `--add-dir` argument to be found. A group of its
		// own is one the supervisor can address, and everything the sandbox starts is in it.
		//
		// Only here: a session at a keyboard is in the terminal's foreground group already,
		// which is what delivers the interrupt, and moving it out would leave it stopped on
		// its first read from the terminal.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		events, pipeErr := cmd.StdoutPipe()
		if pipeErr != nil {
			return row{}, pipeErr
		}
		if err = cmd.Start(); err != nil {
			return row{}, fmt.Errorf("could not start claude: %w", err)
		}
		stop := relay(interrupted, cmd.Process)
		render(events, os.Stdout, l.cwd)
		err = cmd.Wait()
		r.Interrupted = stop()
	}
	r.At = started.UTC().Format(time.RFC3339)
	r.Chain, r.Session = chainID, n
	r.Seconds = int(time.Since(started).Round(time.Second).Seconds())
	r.TimedOut = ctx.Err() != nil
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
	if moved, known := chain.Repo(l.cwd).Moved(before); known {
		r.Moved = &moved
	}
	return r, nil
}

// relay forwards an interrupt to the session and returns a function that stops relaying
// and reports whether one arrived.
//
// The first is forwarded rather than acted on: the session is given the signal it would get
// from a keyboard, so its own shutdown runs and the handoff still lands. The second kills.
// The whole group either way, because the sandbox runs the harness as a child of its own and
// signalling the sandbox alone leaves the session it wrapped running.
func relay(interrupted <-chan os.Signal, p *os.Process) func() bool {
	done, seen := make(chan struct{}), make(chan bool, 1)
	go func() {
		sent := 0
		for {
			select {
			case <-done:
				seen <- sent > 0
				return
			case <-interrupted:
				sent++
				// The second one kills, because the reason to send it twice is that the first
				// did not work. A supervisor that cannot be stopped is worse than a session
				// that dies mid-edit, and the group is what makes the kill reach everything
				// the sandbox started rather than the sandbox alone.
				if sent == 1 {
					signalGroup(p, syscall.SIGINT)
				} else {
					signalGroup(p, syscall.SIGKILL)
				}
			}
		}
	}()
	return func() bool {
		close(done)
		return <-seen
	}
}

// signalGroup sends one signal to everything the session started. A process that has
// already gone is not an error here: the session ending is the outcome being asked for.
func signalGroup(p *os.Process, sig syscall.Signal) {
	if p == nil {
		return
	}
	_ = syscall.Kill(-p.Pid, sig)
}

// runChain runs sessions until the instruction is finished, the chain stops making
// progress, or the bound is reached. Each of the three is a different answer and each
// says so: a chain that stopped for its bound has work left, and one that stalled has a
// handoff to read.
func runChain(l launch, chainDir, chainID, goal string, bound int) (int, error) {
	inherit := chain.LatestHandoff(chainDir)
	first := chain.NextSession(chainDir)
	var prev string
	if inherit != "" {
		prev = chain.Next(chain.Read(inherit))
	}

	// A chain is one thing to interrupt. The session shares this process group, so Ctrl-C
	// reaches it too — and without this the supervisor reads that death as a finished
	// session and starts the next one.
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt)
	defer signal.Stop(interrupted)

	// A chain that is running has no ending, so the one the run before it left goes first.
	chain.ClearEnding(chainDir)

	// What the repository says, which is the test the handoff cannot be trusted for. `ever`
	// gates it: a chain whose work leaves no trace — a measurement, an investigation — never
	// moves a repository and would stall on its second session, so movement judges a chain
	// only once that chain has shown it moves anything at all. Until then the `Next`
	// comparison below is the only evidence there is.
	ever, still := false, 0

	// record writes how the chain stopped and hands back what the supervisor exits with. A
	// failed write does not change the ending: a chain that finished and could not say so
	// still finished, so the failure is narrated and the code stands.
	record := func(code int, reason chain.Reason, at int, handoff string) (int, error) {
		e := chain.Ending{Reason: reason, Session: at, Handoff: handoff}
		if err := chain.WriteEnding(chainDir, e); err != nil {
			narrate("chain %s could not record how it stopped: %v\n", chainID, err)
		}
		return code, nil
	}

	for n := first; n < first+bound; n++ {
		dir := filepath.Join(chainDir, fmt.Sprintf("%02d", n))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 2, err
		}
		r, err := l.session(dir, n, chainID, goal, inherit, interrupted)
		if err != nil {
			return 2, err
		}
		if err := eval.AppendJSON(filepath.Join(chainDir, "sessions.jsonl"), r); err != nil {
			return 2, err
		}
		body := chain.Read(filepath.Join(dir, chain.HandoffName))
		narrate("session %d — %d tool calls, %d of %d tokens, %ds, %s — next: %s\n",
			n, r.Calls, r.Peak, l.limits.Window, r.Seconds, movement(r.Moved),
			or(r.Next, "nothing recorded"))

		// A session stopped by the clock left whatever it had got to; carrying on from that
		// is guessing, and the chain has already spent its longest session on it.
		if r.TimedOut {
			latest := chain.LatestHandoff(chainDir)
			narrate("chain %s stopped: session %d ran past %s — read %s\n",
				chainID, n, l.timeout, latest)
			return record(1, chain.TimedOut, n, latest)
		}
		if chain.Done(body) {
			narrate("chain %s finished after %s\n", chainID, plural(n-first+1, "session"))
			return record(0, chain.Finished, n, filepath.Join(dir, chain.HandoffName))
		}
		// Two sessions planning the same next step is the shape a chain fails in: it is
		// still writing handoffs, and none of them is progress.
		//
		// A step that was read is what makes two of them comparable. Two handoffs the
		// supervisor could not read are not one plan written twice, and taking them for
		// that stopped a chain with a session of its bound unspent.
		if prev != "" && r.Next == prev {
			narrate("chain %s stopped: this session planned what the last one "+
				"did — read %s\n", chainID, filepath.Join(dir, chain.HandoffName))
			return record(1, chain.Stalled, n, filepath.Join(dir, chain.HandoffName))
		}
		// Two, not one: a session that spends its budget reading before it edits is normal,
		// and what this catches is a chain that has stopped converging rather than a slow
		// session. Measured on a 25-session chain, eight in a row committed nothing while the
		// comparison above stayed silent, because each reworded the same plan.
		switch {
		case r.Moved == nil: // no repository to read, so this test does not vote
		case *r.Moved:
			ever, still = true, 0
		case ever:
			still++
		}
		if still >= 2 {
			narrate("chain %s stopped: %d sessions in a row left the repository as they "+
				"found it — read %s\n", chainID, still, filepath.Join(dir, chain.HandoffName))
			return record(1, chain.Stalled, n, filepath.Join(dir, chain.HandoffName))
		}
		// The session's own reading first: it was the one waiting, and an interrupt during a
		// session is consumed there rather than left for this to find.
		stopped := r.Interrupted
		if !stopped {
			select {
			case <-interrupted:
				stopped = true
			default:
			}
		}
		if stopped {
			narrate("chain %s interrupted — continue with `localcode -resume %s`\n",
				chainID, chainID)
			return record(1, chain.Interrupted, n, chain.LatestHandoff(chainDir))
		}
		prev = r.Next
		// The newest handoff the chain holds, not this session's: a session killed before
		// either it or `session-end.sh` wrote one leaves an empty directory, and pointing
		// the next session at that file hands it nothing at all.
		if h := chain.LatestHandoff(chainDir); h != "" {
			inherit = h
		}
	}
	latest := chain.LatestHandoff(chainDir)
	narrate("chain %s stopped after %s, which is its bound — "+
		"read %s and continue with `localcode -resume %s`\n",
		chainID, plural(bound, "session"), latest, chainID)
	return record(1, chain.Bound, first+bound-1, latest)
}

func plural(n int, thing string) string {
	if n == 1 {
		return "1 " + thing
	}
	return fmt.Sprintf("%d %ss", n, thing)
}

// movement is how a session's effect on the repository reads in the supervisor's line. The
// third case is its own words rather than "unchanged": a chain with no repository to read
// has not been measured, and reporting it as still would be a claim.
func movement(moved *bool) string {
	switch {
	case moved == nil:
		return "no repository read"
	case *moved:
		return "repository moved"
	default:
		return "repository unchanged"
	}
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
//
// How each one ended goes beside its session count, because that is what the reader is
// deciding on: a bounded chain has work left and a stalled one has a handoff to read
// first.
func listChains(state string, out io.Writer) (int, error) {
	ids, err := chain.Chains(state)
	if err != nil {
		return 2, err
	}
	if len(ids) == 0 {
		_, _ = fmt.Fprintln(out, "no chains here yet")
		return 1, nil
	}
	for _, id := range ids {
		dir := filepath.Join(state, "chains", id)
		body := chain.Read(chain.LatestHandoff(dir))
		_, _ = fmt.Fprintf(out, "%s  %-11s  %-11s  %s\n", id, plural(chain.NextSession(dir)-1, "session"),
			ending(dir, body), or(chain.Next(body), "nothing recorded"))
	}
	return 0, nil
}

// ending is the word that goes beside a chain's session count. A chain still running has
// recorded none, and so has one that ran before chains recorded theirs — `finished` is the
// only one of the five its handoff can still answer for.
func ending(dir string, handoff []byte) string {
	if e, ok := chain.ReadEnding(dir); ok {
		return string(e.Reason)
	}
	if chain.Done(handoff) {
		return string(chain.Finished)
	}
	return "unrecorded"
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

	// Carrying on a chain that said it was finished would start a session whose whole
	// inheritance is `Next: none`, which does nothing and writes another one. Refused with
	// the two things that are not nothing.
	if goal == "" && (o.cont || o.resume != "") && chain.Done(chain.Read(chain.LatestHandoff(dir))) {
		return "", "", "", fmt.Errorf("chain %s finished: give a new instruction to carry on "+
			"in it, or `localcode -fork %s` to start again from what it knew", id, id)
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
