// Command handoff runs a feature's topmost unticked box across fresh sessions, until the
// box is ticked or a bound is reached. See harness/claude-code/README.md.
//
// Exit codes are the contract:
//
//	0  the box was ticked
//	1  the run completed and the box is still unticked
//	2  the run could not be carried out
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sanyatihy/localcode/internal/eval"
	"github.com/sanyatihy/localcode/internal/handoff"
	"github.com/sanyatihy/localcode/internal/harness"
)

var errUnticked = errors.New("the box is still unticked")

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "handoff: %v\n", err)
		if errors.Is(err, errUnticked) {
			os.Exit(1)
		}
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

// row is one session. The file of them is the measurement.
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
		tools    = fs.String("tools", "Bash,Edit,Read,Write", "tool set")
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

		// The doc is the record, not what the session says about itself.
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
			// Working state inside a finished box is a stale instruction to the next one.
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
	// Configuration paths are relative to the checkout, which is a worktree of its own.
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

// runSession runs one session and reads back what it cost. A failed session is a row, not
// an error: it spent the window this measures either way.
func runSession(cfg config, session int, box string) (row, error) {
	env, err := harness.EnvFromFile(cfg.env)
	if err != nil {
		return row{}, err
	}
	// Its own directory, so nothing reaches the next session but the handoff, and the
	// transcript in it is this session's.
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

	// Flags are the whole configuration: a checkout set up for the editor carries the
	// same variables, and a row could not say which produced it.
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

	// Read whether the session succeeded or not: one stopped by its budget is the case
	// whose peak is worth having.
	if info, err := os.Stat(filepath.Join(cfg.checkout, "HANDOFF.md")); err == nil {
		r.HandoffSize = int(info.Size())
	}

	transcript, err := newestTranscript(state)
	if err != nil {
		// A row, not a raise: the loop runs to the bound.
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

// newestTranscript finds the session's transcript. The state directory is fresh, so there
// is normally one; the newest is taken in case the harness wrote more. A path that cannot
// be stat'd is skipped rather than compared — the previous sort dereferenced a nil FileInfo.
func newestTranscript(state string) (string, error) {
	paths, err := filepath.Glob(filepath.Join(state, "projects", "*", "*.jsonl"))
	if err != nil {
		return "", fmt.Errorf("look for a transcript under %s: %w", state, err)
	}
	newest, at := "", time.Time{}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if newest == "" || info.ModTime().After(at) {
			newest, at = path, info.ModTime()
		}
	}
	if newest == "" {
		return "", fmt.Errorf("no transcript under %s: the session wrote none", state)
	}
	return newest, nil
}

func record(path string, r row) error {
	if path == "" {
		return nil
	}
	return eval.AppendJSON(path, r)
}

// tail keeps the last n bytes, cut on a rune boundary: this carries a command's own error
// text, and a cut through a multi-byte rune corrupts the one thing it is showing.
func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "…" + strings.ToValidUTF8(s[len(s)-n:], "")
}
