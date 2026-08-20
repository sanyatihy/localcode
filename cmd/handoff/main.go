// Command handoff runs one feature's topmost unticked box across fresh sessions, until the
// box is ticked or a bound is reached.
//
// A local model's usable window is smaller than a feature, so the thing that has to fit in
// it is a session rather than the work. Each session starts near the harness's preamble
// floor and is handed the previous one's working state by the SessionStart hook, instead of
// inheriting a conversation it would have to summarise to carry.
//
// This bounds how long a session lives and nothing else. What a session may do is the
// permission mode and the tool set it is given, which are flags with the same defaults a
// session started by hand has.
//
// Exit codes are the contract, so a measurement script can branch without parsing output:
//
//	0  the box was ticked
//	1  the run completed and the box is still unticked
//	2  the run could not be carried out (bad flags, unreadable doc, no transcript)
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sanyatihy/localcode/internal/handoff"
	"github.com/sanyatihy/localcode/internal/harness"
)

var errUnticked = errors.New("the box is still unticked")

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, errUnticked) {
			fmt.Fprintf(os.Stderr, "handoff: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "handoff: %v\n", err)
		os.Exit(2)
	}
}

type config struct {
	doc        string
	checkout   string
	prompt     string
	bin        string
	tools      string
	permission string
	env        string
	settings   string
	results    string
	refusals   string
	sessions   int
	budget     time.Duration
	keep       bool
}

// row is one session, and the file of them is the measurement: whether bounded sessions
// finish a box at all, and what each one cost getting there.
type row struct {
	At          string  `json:"at"`
	Doc         string  `json:"doc"`
	Box         string  `json:"box"`
	Session     int     `json:"session"`
	SessionID   string  `json:"session_id"`
	Peak        int     `json:"peak_context_tokens"`
	Turns       int     `json:"turns"`
	Seconds     float64 `json:"seconds"`
	Refusals    int     `json:"compactions_refused"`
	HandoffSize int     `json:"handoff_bytes"`
	Ticked      bool    `json:"ticked"`
	Err         string  `json:"error,omitempty"`
}

func run(args []string, stdout, stderr *os.File) error {
	fs := flag.NewFlagSet("handoff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		doc      = fs.String("doc", "", "feature doc whose topmost unticked box is the work")
		checkout = fs.String("checkout", ".", "checkout the sessions run in")
		prompt   = fs.String("prompt", "", "what every session is told; empty builds it from -doc")
		bin      = fs.String("bin", "claude", "harness binary")
		tools    = fs.String("tools", "Bash,Edit,Read,Write", "tool set; the four 0008 measured at 3,711 tokens")
		perm     = fs.String("permission-mode", "acceptEdits", "permission mode each session runs under")
		env      = fs.String("claude-code-env", "harness/claude-code/claude-code.env", "environment file pointing the harness at the endpoint")
		settings = fs.String("settings", "harness/claude-code/hooks.json", "settings file carrying the handoff hooks")
		results  = fs.String("results", "", "append a JSONL row per session here; empty writes none")
		refusals = fs.String("refusals", "results/precompact.jsonl", "where the PreCompact hook records what it refused")
		sessions = fs.Int("sessions", 5, "bound: how many sessions the box may take")
		budget   = fs.Duration("budget", 30*time.Minute, "bound: how long one session may run")
		keep     = fs.Bool("keep", false, "leave each session's state directory in place and print it")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg := config{
		doc: *doc, checkout: *checkout, prompt: *prompt, bin: *bin, tools: *tools,
		permission: *perm, env: *env, settings: *settings, results: *results,
		refusals: *refusals, sessions: *sessions, budget: *budget, keep: *keep,
	}
	if err := cfg.validate(); err != nil {
		return err
	}

	for session := 1; session <= cfg.sessions; session++ {
		boxes, err := readBoxes(cfg.doc)
		if err != nil {
			return err
		}
		box, ok := handoff.Topmost(boxes)
		if !ok {
			_, _ = fmt.Fprintf(stdout, "every box in %s is ticked; nothing to run\n", cfg.doc)
			return nil
		}
		if session == 1 {
			_, _ = fmt.Fprintf(stdout, "box: %s\n", box.Text)
		}

		r, err := runSession(cfg, session, box.Text)
		if err != nil {
			return err
		}

		// Asked of the doc rather than of the session: a session that says it finished
		// and did not tick the box has not finished, and the box is the record.
		after, err := readBoxes(cfg.doc)
		if err != nil {
			return err
		}
		r.Ticked = handoff.Ticked(after, box.Text)

		_, _ = fmt.Fprintf(stdout, "session %d  %s  peak %d  turns %d  %s  refused %d\n",
			r.Session, status(r), r.Peak, r.Turns, time.Duration(r.Seconds*float64(time.Second)).Round(time.Second), r.Refusals)
		if err := record(cfg.results, r); err != nil {
			return err
		}
		if r.Ticked {
			// The handoff is working state inside the box that has just been ticked, so
			// it is now a stale instruction: left in place, the first session on the next
			// box would be handed the last one's.
			if err := os.Remove(filepath.Join(cfg.checkout, "HANDOFF.md")); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("%w after %d sessions", errUnticked, cfg.sessions)
}

func status(r row) string {
	switch {
	case r.Ticked:
		return "TICKED"
	case r.Err != "":
		return "ERROR "
	default:
		return "ONWARD"
	}
}

func (c *config) validate() error {
	if c.doc == "" {
		return errors.New("-doc names the feature doc whose box is the work")
	}
	if c.sessions < 1 {
		return fmt.Errorf("-sessions is %d: a bound below one runs nothing", c.sessions)
	}
	abs, err := filepath.Abs(c.checkout)
	if err != nil {
		return err
	}
	c.checkout = abs
	// Every path a session is configured from is read relative to the checkout it runs
	// in, because a feature is worked in a worktree of its own and the flags name what
	// that worktree committed.
	for _, p := range []*string{&c.env, &c.settings, &c.refusals} {
		if !filepath.IsAbs(*p) {
			*p = filepath.Join(c.checkout, *p)
		}
	}
	if !filepath.IsAbs(c.doc) {
		c.doc = filepath.Join(c.checkout, c.doc)
	}
	if c.prompt == "" {
		rel, err := filepath.Rel(c.checkout, c.doc)
		if err != nil {
			rel = c.doc
		}
		c.prompt = fmt.Sprintf("Do the topmost unticked box in %s, and only that one. "+
			"AGENTS.md is the protocol.", rel)
	}
	return nil
}

func readBoxes(doc string) ([]handoff.Box, error) {
	b, err := os.ReadFile(doc)
	if err != nil {
		return nil, fmt.Errorf("feature doc: %w", err)
	}
	boxes := handoff.Boxes(b)
	if len(boxes) == 0 {
		return nil, fmt.Errorf("%s has no ## Tasks boxes", doc)
	}
	return boxes, nil
}

// runSession runs one session to completion and reads back what it cost. A session that
// failed is a row rather than an error: the bound is the driver's, and a harness that died
// on its own has still spent the window this is measuring.
func runSession(cfg config, session int, box string) (row, error) {
	env, err := harness.EnvFromFile(cfg.env)
	if err != nil {
		return row{}, err
	}
	// A directory of its own per session, so nothing a session learned reaches the next
	// one except through HANDOFF.md — and so the transcript it wrote is the only one here.
	state, err := os.MkdirTemp("", fmt.Sprintf("handoff-session-%d-", session))
	if err != nil {
		return row{}, err
	}
	if !cfg.keep {
		defer func() { _ = os.RemoveAll(state) }()
	}
	env = append(env, "CLAUDE_CONFIG_DIR="+state, "PWD="+cfg.checkout)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.budget)
	defer cancel()

	// The session is configured by the flags and by nothing else. A checkout that has
	// been set up for the editor carries the same variables in its own settings file, and
	// leaving that source on means a row cannot say which of the two produced it.
	cmd := exec.CommandContext(ctx, cfg.bin, "-p",
		"--settings", cfg.settings,
		"--setting-sources", "",
		"--tools", cfg.tools,
		"--permission-mode", cfg.permission,
		cfg.prompt)
	cmd.Dir = cfg.checkout
	cmd.Env = env

	started := time.Now()
	out, runErr := cmd.CombinedOutput()
	r := row{
		At:      started.UTC().Format(time.RFC3339),
		Doc:     cfg.doc,
		Box:     box,
		Session: session,
		Seconds: time.Since(started).Seconds(),
	}
	if runErr != nil {
		r.Err = fmt.Sprintf("%v: %s", runErr, tail(string(out), 200))
	}
	if cfg.keep {
		r.Err = strings.TrimSpace(r.Err + " state=" + state)
	}

	// The transcript is read whether the session succeeded or not: a session stopped by
	// its budget is exactly the case whose peak context is worth having.
	if info, err := os.Stat(filepath.Join(cfg.checkout, "HANDOFF.md")); err == nil {
		r.HandoffSize = int(info.Size())
	}

	transcript, err := newestTranscript(state)
	if err != nil {
		// Recorded on the row rather than raised: the driver's job is to keep starting
		// sessions until the box is ticked or the bound is reached, and a session that
		// left nothing to read is one row of that and not the end of the run.
		r.Err = strings.TrimSpace(r.Err + " " + err.Error())
		return r, nil
	}
	s, err := handoff.ReadSession(transcript)
	if err != nil {
		return r, err
	}
	r.SessionID, r.Turns, r.Peak = s.ID, s.Turns, s.Peak
	r.Refusals = handoff.Refusals(cfg.refusals, s.ID)
	return r, nil
}

// newestTranscript finds the session's transcript under its own state directory. The
// directory is fresh, so there is normally one; the newest is taken rather than the only
// one because a harness is free to write more than the session that was asked for.
func newestTranscript(state string) (string, error) {
	paths, err := filepath.Glob(filepath.Join(state, "projects", "*", "*.jsonl"))
	if err != nil || len(paths) == 0 {
		return "", fmt.Errorf("no transcript under %s: the session wrote none", state)
	}
	sort.Slice(paths, func(i, j int) bool {
		a, _ := os.Stat(paths[i])
		b, _ := os.Stat(paths[j])
		return a.ModTime().Before(b.ModTime())
	})
	return paths[len(paths)-1], nil
}

func record(path string, r row) error {
	if path == "" {
		return nil
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
