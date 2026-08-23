package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sanyatihy/localcode/internal/chain"
	"github.com/sanyatihy/localcode/internal/handoff"
)

// account is what one session cost, read off the files it left behind. The supervisor's
// clock and the transcript's counters side by side: the first is the whole session and the
// second only what happened inside a call to the model, so the gap between them is what
// the tools and the harness spent.
type account struct {
	session int
	seconds int // the supervisor's clock for the whole session
	calls   []handoff.Request
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
func accountHere(w io.Writer, id string) (int, error) {
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
	accounts, err := readAccounts(dir)
	if err != nil {
		return 2, err
	}
	if len(accounts) == 0 {
		return 2, fmt.Errorf("chain %s has recorded no session to account for", id)
	}
	printChain(w, id, accounts)
	return 0, nil
}

// readAccounts reads one account per session the chain has recorded. `sessions.jsonl` is
// what says a session happened at all, because the wall clock is the supervisor's and
// nothing the session wrote carries it.
func readAccounts(dir string) ([]account, error) {
	body, err := os.ReadFile(filepath.Join(dir, "sessions.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("read the chain's sessions: %w", err)
	}
	var accounts []account
	for _, line := range bytes.Split(body, []byte("\n")) {
		var r row
		if err := json.Unmarshal(line, &r); err != nil {
			continue // the trailing newline, and any row this does not model
		}
		a := account{session: r.Session, seconds: r.Seconds}
		if t := newestTranscript(filepath.Join(dir, fmt.Sprintf("%02d", r.Session)), os.Environ()); t != "" {
			if a.calls, err = handoff.Requests(t); err != nil {
				return nil, err
			}
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
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
	_, _ = fmt.Fprintf(w, "chain %s — %s, %d s\n", id, plural(len(accounts), "session"), seconds)
	_, _ = fmt.Fprintf(w, "  generated  %d tokens\n", generated)
	_, _ = fmt.Fprintf(w, "  ingested   %d tokens, %d of it preamble — paid once per session\n",
		ingested, preamble)
	_, _ = fmt.Fprintf(w, "  reused     %d tokens the server already held\n", cached)
	_, _ = fmt.Fprintf(w, "  clock      %.1f s inside a call to the model, %.1f s outside one\n",
		model, float64(seconds)-model)
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
		_, _ = fmt.Fprintf(w, "  %7d %6d %8d %10.1f %9d %9d %9d %10d %8s\n", a.session,
			len(a.calls), a.seconds, seconds, a.preamble(), ingested, cached, generated, decode)
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
