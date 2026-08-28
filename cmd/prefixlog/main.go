// Command prefixlog reads a llama-server log and reports what each request it served was
// charged: the prompt, how much of it was ingested, and how much the server already held.
//
// It exists because the probe cannot measure a real session. A harness decides what to
// send and accounts for it in units of its own, so for traffic nobody scripted the
// server's log is the only per-request record — and the turns that ingest from zero are
// the ones worth finding.
//
// One quantity is derived rather than printed, so the tool can be held to a known answer:
// -check reads rows the endpoint itself reported for the same run and names every request
// where the two disagree. A run of cmd/prefixprobe is what to check against, since there
// every prompt is known to the token.
//
// Exit codes are the contract:
//
//	0  the log was read, and under -check the two accounts agree
//	1  under -check they disagree, and every disagreement is printed
//	2  the log could not be read
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sanyatihy/localcode/internal/build"
	"github.com/sanyatihy/localcode/internal/eval"
	"github.com/sanyatihy/localcode/internal/prefix"
)

var errDisagrees = errors.New("the log and the endpoint do not agree")

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, errDisagrees) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "prefixlog: %v\n", err)
		os.Exit(2)
	}
}

func run(args []string, stdout, stderr *os.File) error {
	fs := flag.NewFlagSet("prefixlog", flag.ContinueOnError)
	version := fs.Bool("version", false, "print which build this is, and exit")
	fs.SetOutput(stderr)
	var (
		logPath = fs.String("log", "", "llama-server log to read")
		config  = fs.String("config", "unlabelled", "serving config the log was produced under")
		session = fs.String("session", "unlabelled", "what was driving the endpoint")
		results = fs.String("results", "", "append JSONL rows here; empty writes none")
		check   = fs.String("check", "", "rows the endpoint reported for the same run, to hold the log's account against")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Before anything else is read: a build that cannot say what it is has nothing
	// to say about a machine either.
	if *version {
		_, _ = fmt.Fprintln(stdout, build.Version("prefixlog"))
		return nil
	}
	if *logPath == "" {
		return errors.New("-log is required")
	}
	f, err := os.Open(*logPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	rows, impossible, err := prefix.Requests(f, *config, *session)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("%s carries no completed request", *logPath)
	}

	var ingested, cached, prompt int
	for _, r := range rows {
		if *results != "" {
			if err := eval.AppendJSON(*results, r); err != nil {
				return fmt.Errorf("cannot append result: %w", err)
			}
		}
		prompt += r.PromptTokens
		ingested += r.IngestedTokens
		cached += r.CachedTokens
		_, _ = fmt.Fprintf(stdout, "  %3d  task %-5d %-4s prompt %6d  ingested %6d  cached %6d  hit %5.1f%%  %5.1fs\n",
			r.Index, r.TaskID, r.SlotSelection, r.PromptTokens, r.IngestedTokens, r.CachedTokens,
			100*r.HitShare, r.PromptSeconds)
	}
	// Requests that ingested their whole prompt are what the reading is for, so they are
	// counted here rather than left for a reader to pick out of the rows.
	cold := 0
	for _, r := range rows {
		if r.CachedTokens == 0 {
			cold++
		}
	}
	_, _ = fmt.Fprintf(stdout, "%d requests: %d prompt tokens, %d ingested, %d reused (%.1f%%), %d ingested from zero\n",
		len(rows), prompt, ingested, cached, 100*float64(cached)/float64(max(prompt, 1)), cold)
	// Named rather than folded into the count above: a log this could not account for is a
	// fact about the log, and a total that quietly covers fewer requests than the file holds
	// is the failure the derived figure makes possible.
	if impossible > 0 {
		_, _ = fmt.Fprintf(stdout, "%d request(s) dropped: their account could not have "+
			"happened, so they are in no total above\n", impossible)
	}

	if *check == "" {
		return nil
	}
	want, err := loadRows(*check)
	if err != nil {
		return err
	}
	bad := prefix.Check(rows, want)
	if len(bad) == 0 {
		_, _ = fmt.Fprintf(stdout, "checked against %d rows the endpoint reported: they agree\n", len(want))
		return nil
	}
	for _, d := range bad {
		_, _ = fmt.Fprintf(stdout, "  %s\n", d)
	}
	return fmt.Errorf("%w: %d of %d requests", errDisagrees, len(bad), len(rows))
}

func loadRows(path string) ([]prefix.Row, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []prefix.Row
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r prefix.Row
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, i+1, err)
		}
		out = append(out, r)
	}
	return out, nil
}
