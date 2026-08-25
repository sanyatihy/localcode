package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/sanyatihy/localcode/internal/chain"
	"github.com/sanyatihy/localcode/internal/eval"
	"github.com/sanyatihy/localcode/internal/handoff"
	"github.com/sanyatihy/localcode/internal/harness"
)

// account is what one session cost, read off the files it left behind. The supervisor's
// clock and the transcript's counters side by side: the first is the whole session and the
// second only what happened inside a call to the model, so the gap between them is what
// the tools and the harness spent.
type account struct {
	session int
	at      string // when the supervisor started it, and empty while it runs
	seconds int    // the supervisor's clock for the whole session, and zero while it runs
	running bool   // no row in `sessions.jsonl` yet, so the session has not ended
	limits  chain.Limits
	calls   []handoff.Request
}

// peak is the largest context any call reached, which is the number a ceiling exists to
// keep under the window — and the one that says whether a shorter context would have fit.
// Derived from the calls rather than read from `sessions.jsonl`, so a running session has
// it too.
func (a account) peak() int {
	peak := 0
	for _, c := range a.calls {
		if total := c.Ingest + c.Cached + c.Output; total > peak {
			peak = total
		}
	}
	return peak
}

// preamble is the first call's prompt: at that point the session has said nothing, so what
// it ingested is what the harness put in front of it — and the next session in the chain
// starts by paying it again. Zero when there is no transcript to read.
func (a account) preamble() int {
	if len(a.calls) == 0 {
		return 0
	}
	return a.calls[0].Ingest
}

// totals sums what the calls cost. Seconds is time inside a call and not the session's own
// clock, which the supervisor measured and this cannot.
func (a account) totals() (ingested, cached, generated int, seconds float64) {
	for _, c := range a.calls {
		ingested += c.Ingest
		cached += c.Cached
		generated += c.Output
		seconds += c.Latency.Seconds()
	}
	return ingested, cached, generated, seconds
}

// accountHere reports what one chain of this repository cost. The newest by default,
// because that is the one an operator has just watched run.
func accountHere(w io.Writer, id, jsonl string) (int, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return 2, fmt.Errorf("no working directory: %w", err)
	}
	state, err := repoState(cwd)
	if err != nil {
		return 2, err
	}
	if id == "" {
		ids, err := chain.Chains(state)
		if err != nil {
			return 2, err
		}
		if len(ids) == 0 {
			_, _ = fmt.Fprintln(w, "no chains here yet")
			return 1, nil
		}
		id = ids[0]
	}
	dir := filepath.Join(state, "chains", id)
	if _, err := os.Stat(dir); err != nil {
		return 2, fmt.Errorf("no chain %s here: `localcode sessions` lists them", id)
	}
	rec, err := harness.NewRecorder("claude-code")
	if err != nil {
		return 2, err
	}
	accounts, err := readAccounts(dir, rec)
	if err != nil {
		return 2, err
	}
	if len(accounts) == 0 {
		return 2, fmt.Errorf("chain %s has no session to account for", id)
	}
	printChain(w, id, accounts)
	if jsonl != "" {
		if err := writeRows(jsonl, id, accounts); err != nil {
			return 2, err
		}
	}
	return 0, nil
}

// readAccounts reads one account per session the chain holds.
//
// The chain's numbered directories are what says a session exists, not `sessions.jsonl`:
// that row is appended once the session has ended, so a chain read while it works would
// otherwise be missing the session doing the work. What the file adds is the wall clock,
// which is the supervisor's measurement and is in nothing the session itself wrote.
func readAccounts(dir string, rec harness.Recorder) ([]account, error) {
	timed, err := recordedSeconds(dir)
	if err != nil {
		return nil, err
	}
	var accounts []account
	for _, n := range chain.Sessions(dir) {
		r, ended := timed[n]
		a := account{session: n, at: r.At, seconds: r.Seconds, running: !ended}
		sessionDir := filepath.Join(dir, fmt.Sprintf("%02d", n))
		// The budget the session ran under, which is in the spec the supervisor wrote it
		// and in nothing else. Absent for a session nobody budgeted, which is not an error.
		if spec, err := chain.ReadSpec(sessionDir); err == nil {
			a.limits = spec.Limits
		}
		if t := rec.Transcript(sessionDir); t != "" {
			if a.calls, err = handoff.Requests(t); err != nil {
				return nil, err
			}
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

// recordedSeconds is the supervisor's own record per session that has ended. A chain whose
// first session is still running has written no file at all, which is not an error: it is
// the answer that none of them has ended.
func recordedSeconds(dir string) (map[int]row, error) {
	body, err := os.ReadFile(filepath.Join(dir, "sessions.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return map[int]row{}, nil
		}
		return nil, fmt.Errorf("read the chain's sessions: %w", err)
	}
	rows := map[int]row{}
	for _, line := range bytes.Split(body, []byte("\n")) {
		var r row
		if err := json.Unmarshal(line, &r); err != nil {
			continue // the trailing newline, and any row this does not model
		}
		rows[r.Session] = r
	}
	return rows, nil
}

// printChain reports the chain as one thing. Counts before rates, because the counts are
// the server's own and the rates are fitted: a reader who trusts only what was counted
// still has the tokens, the sessions and the clock.
func printChain(w io.Writer, id string, accounts []account) {
	var seconds, preamble, ingested, cached, generated int
	var model float64
	var calls []handoff.Request
	for _, a := range accounts {
		i, c, g, s := a.totals()
		seconds, preamble = seconds+a.seconds, preamble+a.preamble()
		ingested, cached, generated, model = ingested+i, cached+c, generated+g, model+s
		calls = append(calls, a.calls...)
	}
	running := 0
	for _, a := range accounts {
		if a.running {
			running++
		}
	}
	if running > 0 {
		_, _ = fmt.Fprintf(w, "chain %s — %s, %d still running, %d s recorded\n",
			id, plural(len(accounts), "session"), running, seconds)
	} else {
		_, _ = fmt.Fprintf(w, "chain %s — %s, %d s\n", id, plural(len(accounts), "session"), seconds)
	}
	_, _ = fmt.Fprintf(w, "  generated  %d tokens\n", generated)
	_, _ = fmt.Fprintf(w, "  ingested   %d tokens, %d of it preamble — paid once per session\n",
		ingested, preamble)
	_, _ = fmt.Fprintf(w, "  reused     %d tokens the server already held\n", cached)
	if running > 0 {
		// A session that has not ended has no wall clock of its own yet, and what is
		// outside the model calls is the difference between the two.
		_, _ = fmt.Fprintf(w, "  clock      %.1f s inside a call to the model, and %s "+
			"still to be timed\n", model, plural(running, "session"))
	} else {
		_, _ = fmt.Fprintf(w, "  clock      %.1f s inside a call to the model, %.1f s outside one\n",
			model, float64(seconds)-model)
	}
	if prefill, decode, ok := fitRates(calls); ok {
		_, _ = fmt.Fprintf(w, "  fitted     decode %.2f tok/s, prefill %.1f tok/s, over %d calls\n",
			decode, prefill, len(calls))
	} else {
		_, _ = fmt.Fprintf(w, "  fitted     nothing: %s cannot separate prefill from decode\n",
			plural(len(calls), "call"))
	}
	if len(accounts) > 1 {
		printSessions(w, accounts)
	}
}

// printSessions repeats the chain's own columns per session, which is where a chain that
// went wrong says so: a session is what a handoff, a budget and a preamble are each paid
// per, so a chain's totals hide the one that spent them badly.
func printSessions(w io.Writer, accounts []account) {
	_, _ = fmt.Fprintf(w, "\n  %7s %6s %8s %10s %9s %9s %9s %10s %8s\n", "session", "calls",
		"seconds", "in a call", "preamble", "ingested", "reused", "generated", "decode")
	for _, a := range accounts {
		ingested, cached, generated, seconds := a.totals()
		// A session whose own calls do not decide a rate says so rather than borrowing the
		// chain's: the chain's fit is over other sessions' calls as well.
		decode := "—"
		if _, fitted, ok := fitRates(a.calls); ok {
			decode = fmt.Sprintf("%.2f", fitted)
		}
		clock := fmt.Sprintf("%d", a.seconds)
		if a.running {
			clock = "—"
		}
		_, _ = fmt.Fprintf(w, "  %7d %6d %8s %10.1f %9d %9d %9d %10d %8s\n", a.session,
			len(a.calls), clock, seconds, a.preamble(), ingested, cached, generated, decode)
	}
}

// fitRates splits a call's clock between the prompt it read and the reply it wrote.
//
// Every call cost its prompt at one rate and its reply at another, so over many calls the
// two rates are a least-squares fit through the origin. Derived rather than observed, and
// the only derived numbers here: a transcript records no first-token time, so the boundary
// inside a call is the one thing its counters do not state.
//
// Not ok when the calls cannot decide it — one call, or a chain whose prompts and replies
// grew together, where every pair of rates that sums right fits as well as any other.
func fitRates(calls []handoff.Request) (prefill, decode float64, ok bool) {
	var pp, pg, gg, pl, gl float64
	used := 0
	for _, c := range calls {
		l := c.Latency.Seconds()
		if l <= 0 {
			continue // a call the clock did not separate from the row before it
		}
		p, g := float64(c.Ingest), float64(c.Output)
		pp, pg, gg = pp+p*p, pg+p*g, gg+g*g
		pl, gl = pl+p*l, gl+g*l
		used++
	}
	det := pp*gg - pg*pg
	if used < 2 || det <= 1e-9*pp*gg {
		return 0, 0, false
	}
	perPrompt, perReply := (gg*pl-pg*gl)/det, (pp*gl-pg*pl)/det
	if perPrompt <= 0 || perReply <= 0 {
		return 0, 0, false
	}
	return 1 / perPrompt, 1 / perReply, true
}

// dataRow is one line of the JSONL `docs/data/` takes. Field names are 0025's, because the
// two files are read together: a chain accounted here and the same chain measured off the
// server's counters have to be talking about the same tokens.
//
// The rates are the only fitted numbers and `rates_fitted` is what says so — false, and
// they are absent rather than zero, since a zero decode rate reads as a measurement.
type dataRow struct {
	Record    string  `json:"record"`
	Chain     string  `json:"chain"`
	Session   int     `json:"session,omitempty"`
	At        string  `json:"at,omitempty"`
	Running   bool    `json:"running,omitempty"`
	Sessions  int     `json:"sessions,omitempty"`
	Wall      int     `json:"wall_seconds"`
	Model     float64 `json:"model_seconds"`
	Calls     int     `json:"model_calls"`
	Preamble  int     `json:"preamble_tokens"`
	Ingested  int     `json:"prompt_tokens_ingested"`
	Cached    int     `json:"prompt_tokens_cached"`
	Generated int     `json:"tokens_generated"`
	Peak      int     `json:"peak_context_tokens"`
	Window    int     `json:"window_tokens,omitempty"`
	Ceiling   int     `json:"ceiling_tokens,omitempty"`
	Fitted    bool    `json:"rates_fitted"`
	Decode    float64 `json:"decode_per_second,omitempty"`
	Prompt    float64 `json:"prompt_per_second,omitempty"`
}

// writeRows appends the chain and then a row per session, which is the order they are read
// in. Appended rather than written: a results file here accumulates across runs, and one
// that truncated would lose the chain it was compared against.
func writeRows(path, id string, accounts []account) error {
	chainRow := dataRow{Record: "chain", Chain: id, Sessions: len(accounts)}
	var calls []handoff.Request
	rows := []dataRow{{}}
	for _, a := range accounts {
		ingested, cached, generated, seconds := a.totals()
		r := dataRow{
			Record: "session", Chain: id, Session: a.session, At: a.at, Running: a.running,
			Wall: a.seconds, Model: round(seconds, 1), Calls: len(a.calls),
			Preamble: a.preamble(), Ingested: ingested, Cached: cached, Generated: generated,
			Peak: a.peak(), Window: a.limits.Window, Ceiling: a.limits.Ceiling,
		}
		r.Prompt, r.Decode, r.Fitted = fittedRates(a.calls)
		rows = append(rows, r)

		chainRow.Wall += a.seconds
		chainRow.Model += seconds
		chainRow.Calls += len(a.calls)
		chainRow.Preamble += a.preamble()
		chainRow.Ingested += ingested
		chainRow.Cached += cached
		chainRow.Generated += generated
		if p := a.peak(); p > chainRow.Peak {
			chainRow.Peak = p
		}
		calls = append(calls, a.calls...)
	}
	chainRow.Model = round(chainRow.Model, 1)
	chainRow.Prompt, chainRow.Decode, chainRow.Fitted = fittedRates(calls)
	rows[0] = chainRow
	for _, r := range rows {
		if err := eval.AppendJSON(path, r); err != nil {
			return err
		}
	}
	return nil
}

// fittedRates is fitRates rounded to what the fit can claim. Two decimals on a decode rate
// and one on a prompt rate, which is the precision the published figures here carry.
func fittedRates(calls []handoff.Request) (prompt, decode float64, ok bool) {
	p, d, ok := fitRates(calls)
	if !ok {
		return 0, 0, false
	}
	return round(p, 1), round(d, 2), true
}

func round(v float64, places int) float64 {
	scale := math.Pow(10, float64(places))
	return math.Round(v*scale) / scale
}
