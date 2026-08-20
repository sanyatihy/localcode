package prefix

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sanyatihy/localcode/internal/eval"
)

func testConversation() Conversation {
	return Conversation{SystemWords: 20, TurnWords: 30, Turns: 3, SmallWords: 10}
}

// The prompt of turn n+1 must contain the prompt of turn n unchanged. That is the whole
// premise of a served prefix: if the conversation rewrote its own history, no cache
// could help it and the measurement would be of the instrument.
func TestTurnGrowsWithoutRewriting(t *testing.T) {
	c := testConversation()
	for n := 1; n < c.Turns; n++ {
		this, next := c.Turn(n), c.Turn(n+1)
		if len(next) <= len(this) {
			t.Fatalf("turn %d has %d messages, turn %d has %d: it did not grow", n, len(this), n+1, len(next))
		}
		for i, m := range this {
			if next[i] != m {
				t.Fatalf("turn %d rewrote message %d of turn %d", n+1, i, n)
			}
		}
	}
}

// Two runs of the same conversation must send the same bytes, or two conditions cannot
// be compared and a repeat is not a repeat.
func TestConversationIsDeterministic(t *testing.T) {
	a, b := testConversation().Turn(3), testConversation().Turn(3)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("message %d differs between two builds of the same conversation", i)
		}
	}
}

// The small call must share no prefix with the conversation, or it would be served from
// the conversation's cache and disturb nothing — the run would report a null result for
// a reason that has nothing to do with the server.
func TestSmallCallSharesNoPrefix(t *testing.T) {
	c := testConversation()
	if c.Small(1)[0].Content == c.Turn(1)[0].Content {
		t.Fatal("the small call opens with the conversation's system prompt")
	}
	if c.Small(1)[1].Content == c.Small(2)[1].Content {
		t.Fatal("the small call is identical at every position; a harness's are not")
	}
}

// usage is what the fake server reports back, in the Messages API's units: input_tokens
// counts only what was ingested, and the cache read is reported beside it.
type usage struct{ ingested, cached int }

func fakeServer(t *testing.T, reply func(system string, n int) usage) *httptest.Server {
	t.Helper()
	n := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			System string `json:"system"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		u := reply(req.System, n)
		n++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens": u.ingested, "cache_read_input_tokens": u.cached, "output_tokens": 1,
			},
		})
	}))
}

func testClient(url string) *eval.Client {
	c := eval.NewClient(url, 5*time.Second)
	c.API = eval.APIMessages
	return c
}

func TestRunRecordsOneRowPerRequest(t *testing.T) {
	srv := fakeServer(t, func(_ string, _ int) usage { return usage{ingested: 100, cached: 900} })
	defer srv.Close()

	rows, err := Run(context.Background(), testClient(srv.URL), testConversation(),
		Options{Config: "test", Interleave: true, MaxTokens: 4,
			Props: eval.ServerProps{NCtx: 4096, ModelPath: "/models/test.gguf", Available: true}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// Three turns with a small call after all but the last: the last one has nothing
	// left to disturb, so making it would cost a request and measure nothing.
	want := []struct {
		kind  string
		index int
	}{
		{KindConversation, 1}, {KindSmall, 1},
		{KindConversation, 2}, {KindSmall, 2},
		{KindConversation, 3},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d", len(rows), len(want))
	}
	for i, w := range want {
		if rows[i].Kind != w.kind || rows[i].Index != w.index {
			t.Errorf("row %d is %s %d, want %s %d", i, rows[i].Kind, rows[i].Index, w.kind, w.index)
		}
		if rows[i].Condition != ConditionInterleaved {
			t.Errorf("row %d is recorded as %q", i, rows[i].Condition)
		}
		if rows[i].PromptTokens != 1000 || rows[i].IngestedTokens != 100 || rows[i].HitShare != 0.9 {
			t.Errorf("row %d: prompt %d ingested %d hit %v", i,
				rows[i].PromptTokens, rows[i].IngestedTokens, rows[i].HitShare)
		}
		if rows[i].ServedNCtx != 4096 || rows[i].ServedModel != "test.gguf" {
			t.Errorf("row %d does not carry what served it: %d %q", i, rows[i].ServedNCtx, rows[i].ServedModel)
		}
	}
}

func TestRunWithoutInterleaveMakesOnlyConversationRequests(t *testing.T) {
	srv := fakeServer(t, func(_ string, _ int) usage { return usage{ingested: 10, cached: 0} })
	defer srv.Close()

	rows, err := Run(context.Background(), testClient(srv.URL), testConversation(), Options{MaxTokens: 4})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want one per turn", len(rows))
	}
	for _, r := range rows {
		if r.Kind != KindConversation || r.Condition != ConditionClean {
			t.Fatalf("clean run produced a %s row recorded as %q", r.Kind, r.Condition)
		}
	}
}

// A server that reports no accounting must stop the run rather than fill the file with
// turns that appear to have cost nothing.
func TestRunRefusesAnUnaccountedRequest(t *testing.T) {
	srv := fakeServer(t, func(_ string, _ int) usage { return usage{} })
	defer srv.Close()

	rows, err := Run(context.Background(), testClient(srv.URL), testConversation(), Options{MaxTokens: 4})
	if err == nil {
		t.Fatal("a request the server did not account for was recorded as a row")
	}
	if len(rows) != 0 {
		t.Fatalf("got %d rows from a run that could not be measured", len(rows))
	}
	if !strings.Contains(err.Error(), "conversation 1") {
		t.Errorf("error does not say which request failed: %v", err)
	}
}

// A small call may share the conversation's system prompt, and a run that measures that
// traffic must not be recorded under the same condition as one that does not: the server
// selects a slot differently for the two.
func TestSharedSmallCallOpensWithTheConversation(t *testing.T) {
	c := testConversation()
	c.SmallSharesSystem = true
	if c.Small(1)[0].Content != c.Turn(1)[0].Content {
		t.Fatal("the shared small call does not open with the conversation's system prompt")
	}
	if c.Small(1)[1].Content == c.Turn(1)[1].Content {
		t.Fatal("the shared small call sends the conversation's own turn, not a call beside it")
	}

	srv := fakeServer(t, func(_ string, _ int) usage { return usage{ingested: 10, cached: 90} })
	defer srv.Close()
	rows, err := Run(context.Background(), testClient(srv.URL), c, Options{Interleave: true, MaxTokens: 4})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, r := range rows {
		if r.Condition != ConditionInterleavedShared {
			t.Fatalf("row recorded as %q, want %q", r.Condition, ConditionInterleavedShared)
		}
		if !r.Conversation.SmallSharesSystem {
			t.Fatal("the row does not carry which traffic it measured")
		}
	}
}

// The hit share is the conversation's. A small call has no prefix to reuse by
// construction, so averaging it in would report the instrument rather than the session.
func TestSummariseSeparatesTheCallsBeside(t *testing.T) {
	got := Summarise([]Row{
		{Kind: KindConversation, PromptTokens: 1000, IngestedTokens: 1000, CachedTokens: 0, WallSeconds: 10},
		{Kind: KindSmall, PromptTokens: 500, IngestedTokens: 500, WallSeconds: 5},
		{Kind: KindConversation, PromptTokens: 2000, IngestedTokens: 1000, CachedTokens: 1000, WallSeconds: 10},
	})
	want := Summary{
		Turns: 2, PromptTokens: 3000, IngestedTokens: 2000, CachedTokens: 1000,
		HitShare: 1000.0 / 3000.0, TurnsHit: 1, WallSeconds: 20, SmallWallSeconds: 5,
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// Each row must reach the caller as it is taken, not when the run ends: a condition is
// tens of minutes of machine time, and a run that dies at the last turn must not take
// the earlier ones with it. Counting the server's requests against the rows delivered
// is what tells the two apart — batched at the end, the first row would arrive with
// every request already made.
func TestRunRecordsEachRowAsItIsTaken(t *testing.T) {
	var served atomic.Int64
	srv := fakeServer(t, func(_ string, _ int) usage {
		served.Add(1)
		return usage{ingested: 10, cached: 90}
	})
	defer srv.Close()

	rows := 0
	_, err := Run(context.Background(), testClient(srv.URL), testConversation(), Options{
		MaxTokens: 4,
		Record: func(Row) error {
			rows++
			if got := served.Load(); got != int64(rows) {
				t.Errorf("row %d arrived with %d requests already made", rows, got)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if rows != 3 {
		t.Fatalf("recorded %d rows, want one per turn", rows)
	}
}

// A row that cannot be written stops the run. Measuring on into a file nothing can be
// written to spends the machine for nothing.
func TestRunStopsWhenARowCannotBeRecorded(t *testing.T) {
	srv := fakeServer(t, func(_ string, _ int) usage { return usage{ingested: 10, cached: 90} })
	defer srv.Close()

	calls := 0
	rows, err := Run(context.Background(), testClient(srv.URL), testConversation(), Options{
		MaxTokens: 4,
		Record:    func(Row) error { calls++; return errors.New("disk is gone") },
	})
	if err == nil {
		t.Fatal("the run continued past a row it could not record")
	}
	if calls != 1 || len(rows) != 1 {
		t.Fatalf("recorded %d rows and made %d attempts after the first failure", len(rows), calls)
	}
}
