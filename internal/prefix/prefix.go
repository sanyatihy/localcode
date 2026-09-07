// Package prefix measures what a conversation re-ingests when it shares an endpoint with
// traffic that is not part of it. On this hardware a turn costs its prompt and not its
// answer, so what a session is worth turns on how much of each prompt the server still
// holds when the turn arrives.
//
// It replays a fixed conversation rather than driving a harness: only a scripted one
// repeats. serverlog.go reads the same account for traffic nobody scripted.
package prefix

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	"github.com/sanyatihy/localcode/internal/build"
	"github.com/sanyatihy/localcode/internal/eval"
)

// The two conditions a run is recorded under. They differ in one thing: whether
// anything else uses the endpoint between turns.
const (
	ConditionClean       = "clean"
	ConditionInterleaved = "interleaved"

	// A small call that opens with the conversation's own system prompt is the other
	// shape side traffic comes in, and the server treats it differently: it shares
	// enough of the prefix to be selected by similarity rather than as the least
	// recently used slot, which is a different path through the server to the same
	// question.
	ConditionInterleavedShared = "interleaved-shared"
)

// The two kinds of request a run makes. A small call is small in the way a harness's
// side traffic is small — a few thousand tokens against the conversation's tens of
// thousands — and shares no prefix with the conversation.
const (
	KindConversation = "conversation"
	KindSmall        = "small"
)

// cannedReply stands in for what the model said: turn n+1's prompt must be byte-identical
// across conditions or their cache figures are not comparable, and a generated reply is
// not. It costs nothing in reuse — the common prefix ends where this reply begins.
const cannedReply = "Understood."

// Conversation is the traffic a probe replays. Sizes are in words because that is what
// the generator can honour exactly; this vocabulary runs at about one token per word,
// which is close enough to place a run in the band an observation came from.
type Conversation struct {
	SystemWords int `json:"system_words"` // the harness's preamble: system prompt and tools
	TurnWords   int `json:"turn_words"`   // one turn's tool output, which grows the prompt
	Turns       int `json:"turns"`
	SmallWords  int `json:"small_words"` // the interleaved call, whose prefix is its own

	// SmallSharesSystem opens the small call with the conversation's system prompt
	// instead of one of its own. It is on the conversation rather than in the run's
	// options because it changes the traffic, and a row that cannot say which traffic
	// it measured is not evidence for either.
	SmallSharesSystem bool `json:"small_shares_system"`
}

// filler is code-shaped rather than prose: a session carries repository text, and its
// tokenisation is not English's. Deliberately not shared with the scorer's vocabulary — a
// change here would otherwise move the prompts behind every recorded retrieval number.
var filler = []string{
	"session", "token", "refresh", "handler", "request", "context", "buffer",
	"index", "commit", "parser", "value", "result", "config", "client", "server",
	"stream", "record", "module", "target", "branch", "queue", "worker", "socket",
}

// words renders n deterministic words. Seeded per call site so adding a turn to the end
// of a conversation does not disturb the text of the ones before it.
func words(seed, n int) string {
	rng := rand.New(rand.NewSource(int64(seed)))
	var b strings.Builder
	for i := range n {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(filler[rng.Intn(len(filler))])
	}
	return b.String()
}

// Turn returns the messages the conversation sends on turn n, 1-based: everything already
// sent plus one more tool result — a growing prefix resent whole, which is the shape a
// real session has.
func (c Conversation) Turn(n int) []eval.Message {
	msgs := []eval.Message{{
		Role:    "system",
		Content: "You are a coding agent working in a repository.\n\n" + words(1, c.SystemWords),
	}}
	for i := 1; i <= n; i++ {
		if i > 1 {
			msgs = append(msgs, eval.Message{Role: "assistant", Content: cannedReply})
		}
		msgs = append(msgs, eval.Message{
			Role:    "user",
			Content: fmt.Sprintf("Tool result %d:\n\n%s", i, words(100+i, c.TurnWords)),
		})
	}
	return msgs
}

// Small returns the call interleaved after turn n. Its body differs per position the way a
// harness's does, and its system prompt is either its own — sharing nothing — or the
// conversation's, which is as much prefix as side traffic can share.
func (c Conversation) Small(n int) []eval.Message {
	system := "Summarise the conversation in a few words."
	if c.SmallSharesSystem {
		system = c.Turn(1)[0].Content
	}
	return []eval.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: "Title this:\n\n" + words(900+n, c.SmallWords)},
	}
}

// Row is one request's record. Ingested and cached are the whole measurement: the server
// reports them per request, and their sum is the prompt that was sent.
type Row struct {
	RunAt     string `json:"run_at"`
	Config    string `json:"config"`    // the serving config under test
	Condition string `json:"condition"` // clean | interleaved | interleaved-shared
	Kind      string `json:"kind"`      // conversation | small

	// Index is the conversation turn this row belongs to. A small call carries the
	// turn it follows, so a row pair is legible without reading the file in order.
	Index int `json:"index"`

	// The conversation that produced the row. Two depths of the same conversation on
	// the same config are the same label otherwise, and averaging them would report a
	// hit rate no session has.
	Conversation Conversation `json:"conversation"`

	PromptTokens   int     `json:"prompt_tokens"`
	IngestedTokens int     `json:"ingested_tokens"`
	CachedTokens   int     `json:"cached_tokens"`
	HitShare       float64 `json:"hit_share"`
	WallSeconds    float64 `json:"wall_seconds"`

	ServedNCtx  int    `json:"served_n_ctx"`
	ServedModel string `json:"served_model"`

	// The stack that served the row: llama.cpp's build string, and the ggml library it
	// opened for the kernels. Both `unknown` when the server could not be asked.
	ServedBuild   string `json:"served_build"`
	ServedBackend string `json:"served_backend"`

	// Driver is the build of this repository that wrote the row: a short revision, with
	// `+modified` when the tree it was built from was not clean, and `unknown` when the
	// build recorded none. A row is evidence only if somebody can get back to the code that
	// produced it, and the driver's own arithmetic has moved under rows before.
	Driver string `json:"driver"`
}

// Options are what a run needs beyond the conversation itself. Props travels with the
// rows rather than being read from them later: a row that cannot say what served it
// cannot be attributed to a config.
type Options struct {
	Config     string
	Props      eval.ServerProps
	Interleave bool

	// MaxTokens is small on purpose. This measures prompt processing, and generation at
	// this size would cost more wall clock than the thing being measured.
	MaxTokens int

	// Record is called with each row as it is measured: a condition is tens of minutes, so
	// a row reaches its file when taken. Returning an error stops the run.
	Record func(Row) error
}

// Run replays the conversation once and returns one row per request it made.
//
// Requests are sequential because the server is: sending the small call concurrently
// would measure the queue rather than the cache.
func Run(ctx context.Context, c *eval.Client, conv Conversation, opt Options) ([]Row, error) {
	condition := ConditionClean
	switch {
	case opt.Interleave && conv.SmallSharesSystem:
		condition = ConditionInterleavedShared
	case opt.Interleave:
		condition = ConditionInterleaved
	}
	var out []Row
	record := func(r Row) error {
		out = append(out, r)
		if opt.Record == nil {
			return nil
		}
		return opt.Record(r)
	}
	for i := 1; i <= conv.Turns; i++ {
		row, err := send(ctx, c, conv.Turn(i), conv, opt, condition, KindConversation, i)
		if err != nil {
			return out, err
		}
		if err := record(row); err != nil {
			return out, err
		}

		// After the last turn there is nothing left for a small call to disturb, so
		// making one would cost a minute and measure nothing.
		if !opt.Interleave || i == conv.Turns {
			continue
		}
		row, err = send(ctx, c, conv.Small(i), conv, opt, condition, KindSmall, i)
		if err != nil {
			return out, err
		}
		if err := record(row); err != nil {
			return out, err
		}
	}
	return out, nil
}

func send(ctx context.Context, c *eval.Client, msgs []eval.Message, conv Conversation,
	opt Options, condition, kind string, index int) (Row, error) {
	resp, err := c.Converse(ctx, msgs, opt.MaxTokens)
	if err != nil {
		return Row{}, fmt.Errorf("%s %d: %w", kind, index, err)
	}
	if len(resp.Error) > 0 {
		return Row{}, fmt.Errorf("%s %d: server: %s", kind, index, resp.Error)
	}
	// A request the server did not account for would read as a turn that ingested
	// nothing, which is the very finding this measures. It stops the run instead.
	if resp.Usage.PromptTokens == 0 {
		return Row{}, fmt.Errorf("%s %d: the server reported no prompt tokens", kind, index)
	}
	cached := resp.Usage.PromptTokensDetails.CachedTokens
	return Row{
		RunAt:          eval.Now().Format(time.RFC3339),
		Driver:         build.Revision(),
		Config:         opt.Config,
		Condition:      condition,
		Kind:           kind,
		Index:          index,
		Conversation:   conv,
		PromptTokens:   resp.Usage.PromptTokens,
		IngestedTokens: resp.Usage.PromptTokens - cached,
		CachedTokens:   cached,
		HitShare:       float64(cached) / float64(resp.Usage.PromptTokens),
		WallSeconds:    resp.Wall.Seconds(),
		ServedNCtx:     opt.Props.NCtx,
		ServedBuild:    opt.Props.BuildOrUnknown(),
		ServedBackend:  opt.Props.BackendOrUnknown(),
		ServedModel:    filepath.Base(opt.Props.ModelPath),
	}, nil
}

// Summary is what a run comes down to. The hit share is the conversation's alone: a
// small call has no prefix to reuse by construction, and averaging it in would report
// the instrument rather than the session.
type Summary struct {
	Turns            int
	PromptTokens     int
	IngestedTokens   int
	CachedTokens     int
	HitShare         float64
	TurnsHit         int // turns that found any of their prompt still served
	WallSeconds      float64
	SmallWallSeconds float64 // what the interleaved calls cost, which the session also pays
}

func Summarise(rows []Row) Summary {
	var s Summary
	for _, r := range rows {
		if r.Kind == KindSmall {
			s.SmallWallSeconds += r.WallSeconds
			continue
		}
		s.Turns++
		s.PromptTokens += r.PromptTokens
		s.IngestedTokens += r.IngestedTokens
		s.CachedTokens += r.CachedTokens
		s.WallSeconds += r.WallSeconds
		if r.CachedTokens > 0 {
			s.TurnsHit++
		}
	}
	if s.PromptTokens > 0 {
		s.HitShare = float64(s.CachedTokens) / float64(s.PromptTokens)
	}
	return s
}
