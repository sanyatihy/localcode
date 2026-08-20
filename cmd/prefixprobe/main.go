// Command prefixprobe replays a fixed conversation against a served endpoint and
// records, per turn, what the server ingested against what it served from its cache.
//
// It exists to answer one question with evidence rather than from a log line: what a
// conversation costs when something else uses the same endpoint between its turns.
// Run it twice against the same server, once with -interleave and once without, and the
// difference is what that traffic costs the session.
//
// The dialect is the Anthropic Messages path, because that is the one an editor session
// takes; sampling and the thinking toggle are the server's on that path, so the config
// under test is the whole configuration and there is nothing to pass here.
//
// Exit codes are the contract:
//
//	0  the run completed and the rows were written
//	2  the run could not be carried out (bad flags, no server, a request that failed)
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sanyatihy/localcode/internal/eval"
	"github.com/sanyatihy/localcode/internal/prefix"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "prefixprobe: %v\n", err)
		os.Exit(2)
	}
}

func run(args []string, stdout, stderr *os.File) error {
	fs := flag.NewFlagSet("prefixprobe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		endpoint = fs.String("endpoint", "http://127.0.0.1:8081", "endpoint the conversation is replayed against")
		config   = fs.String("config", "unlabelled", "label for the serving config under test")
		results  = fs.String("results", "", "append JSONL rows here; empty writes none")
		// The defaults are the session 0018 observed, in words: a preamble, five turns
		// of tool output reaching ~28k, and a side call the size of the ones seen either
		// side of that turn. Changing them is how a run asks about another depth.
		system     = fs.Int("system-words", 3000, "preamble: system prompt and tool definitions")
		turnWords  = fs.Int("turn-words", 5000, "one turn's tool output")
		turns      = fs.Int("turns", 5, "conversation turns")
		small      = fs.Int("small-words", 5500, "the interleaved call")
		interleave = fs.Bool("interleave", false,
			"make a small call between turns, which is the condition under test")
		shared = fs.Bool("small-shares-system", false,
			"open the small call with the conversation's system prompt rather than one of its own")
		maxTokens = fs.Int("max-tokens", 4, "generation cap; this measures prompt processing, not generation")
		timeout   = fs.Duration("timeout", 30*time.Minute, "per-request timeout")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *turns < 1 {
		return fmt.Errorf("-turns %d: a conversation is at least one turn", *turns)
	}

	ctx := context.Background()
	client := eval.NewClient(*endpoint, *timeout)
	client.API = eval.APIMessages

	// Unlike the scorer, this refuses to run without /props. Every row it writes is a
	// claim about one serving config against another, and a run that cannot name what
	// served it cannot support that claim.
	props, err := client.Props(ctx)
	if err != nil {
		return fmt.Errorf("%s does not say what it serves (%w); a cache figure that cannot be "+
			"attributed to a config is not evidence", *endpoint, err)
	}

	conv := prefix.Conversation{
		SystemWords: *system, TurnWords: *turnWords, Turns: *turns, SmallWords: *small,
		SmallSharesSystem: *shared,
	}
	// Each row is written and printed as it is taken. A condition here is tens of
	// minutes, so what was measured before a failure must already be on disk.
	opt := prefix.Options{
		Config: *config, Props: props, Interleave: *interleave, MaxTokens: *maxTokens,
		Record: func(r prefix.Row) error {
			_, _ = fmt.Fprintf(stdout, "  %-12s %d  prompt %6d  ingested %6d  cached %6d  hit %5.1f%%  wall %6.1fs\n",
				r.Kind, r.Index, r.PromptTokens, r.IngestedTokens, r.CachedTokens, 100*r.HitShare, r.WallSeconds)
			if *results == "" {
				return nil
			}
			return eval.AppendJSON(*results, r)
		},
	}
	_, _ = fmt.Fprintf(stdout, "%s at n_ctx %d: %d turns of ~%d words, interleave=%v\n",
		*config, props.NCtx, *turns, *turnWords, *interleave)

	rows, err := prefix.Run(ctx, client, conv, opt)
	if err != nil {
		return err
	}

	s := prefix.Summarise(rows)
	_, _ = fmt.Fprintf(stdout, "%d turns: %d prompt tokens, %d ingested, %d cached (%.1f%%), %d/%d turns hit\n",
		s.Turns, s.PromptTokens, s.IngestedTokens, s.CachedTokens, 100*s.HitShare, s.TurnsHit, s.Turns)
	_, _ = fmt.Fprintf(stdout, "wall: %.1fs in the conversation, %.1fs in the calls beside it\n",
		s.WallSeconds, s.SmallWallSeconds)
	return nil
}
