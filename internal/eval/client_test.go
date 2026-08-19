package eval

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const metricsBody = `# HELP llamacpp:prompt_tokens_total Number of prompt tokens processed.
# TYPE llamacpp:prompt_tokens_total counter
llamacpp:prompt_tokens_total 1616
llamacpp:prompt_tokens_cached_total 3199
llamacpp:prompt_seconds_total 12.5
llamacpp:tokens_predicted_total 165
llamacpp:n_decode_total 165
`

func TestParseMetricsReadsTheCountersThisProjectUses(t *testing.T) {
	got, err := parseMetrics(metricsBody)
	if err != nil {
		t.Fatalf("parseMetrics: %v", err)
	}
	want := ServerMetrics{PromptTokens: 1616, CachedTokens: 3199, PredictedTokens: 165, Available: true}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// A missing counter is an error rather than a zero: a run recorded as costing nothing
// would be read as a harness that spends nothing, which is the comparison's whole subject.
func TestParseMetricsRefusesAnIncompleteExposition(t *testing.T) {
	if _, err := parseMetrics("llamacpp:prompt_tokens_total 12\n"); err == nil {
		t.Fatal("expected an error naming how many counters were found")
	}
}

// Sampled either side of a run, the difference is that run's — but only when both samples
// exist. A server without --metrics must not produce a confident zero.
func TestServerMetricsSubNeedsBothSamples(t *testing.T) {
	before := ServerMetrics{PromptTokens: 10, CachedTokens: 5, PredictedTokens: 2, Available: true}
	after := ServerMetrics{PromptTokens: 40, CachedTokens: 25, PredictedTokens: 12, Available: true}
	got := after.Sub(before)
	want := ServerMetrics{PromptTokens: 30, CachedTokens: 20, PredictedTokens: 10, Available: true}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if spent := after.Sub(ServerMetrics{}); spent.Available {
		t.Errorf("a missing first sample produced %+v", spent)
	}
	if spent := (ServerMetrics{}).Sub(before); spent.Available {
		t.Errorf("a missing second sample produced %+v", spent)
	}
}

// Turns are counted by watching which task the slot is busy with, so the count must be of
// distinct tasks and not of samples: a turn slow enough to be seen twice is still one turn.
func TestTurnCounterCountsTasksNotSamples(t *testing.T) {
	var task atomic.Int64
	task.Store(41)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `[{"id_task":%d,"is_processing":true}]`, task.Load())
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 2*time.Second)
	counter := c.CountTurns(context.Background(), time.Millisecond)
	// Three tasks, each seen over many samples.
	for _, id := range []int64{41, 55, 66} {
		task.Store(id)
		time.Sleep(20 * time.Millisecond)
	}
	if got := counter.Stop(); got != 3 {
		t.Errorf("counted %d turns, want 3", got)
	}
}

// An idle slot is not a turn, and a server that cannot be reached is zero turns rather
// than a hung run.
func TestTurnCounterIgnoresAnIdleSlot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `[{"id_task":7,"is_processing":false}]`)
	}))
	c := NewClient(srv.URL, 2*time.Second)
	counter := c.CountTurns(context.Background(), time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	srv.Close()
	if got := counter.Stop(); got != 0 {
		t.Errorf("counted %d turns on an idle server, want 0", got)
	}
}
