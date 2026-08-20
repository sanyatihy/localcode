package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A server that does not speculate reports no draft counters, and that is not an
// acceptance of zero: the ratio must be able to say "unavailable" instead.
func TestAcceptanceLengthSeparatesUnavailableFromZero(t *testing.T) {
	drafted, accepted := 40, 30
	spec := &Response{}
	spec.Timings.PredictedN = 100
	spec.Timings.DraftN, spec.Timings.DraftNAccepted = &drafted, &accepted

	tau, ok := spec.AcceptanceLength()
	if !ok {
		t.Fatal("acceptance should be available when the server reports drafts")
	}
	// 100 committed tokens over 70 verification steps.
	if want := 100.0 / 70.0; tau != want {
		t.Fatalf("tau = %v, want %v", tau, want)
	}

	plain := &Response{}
	plain.Timings.PredictedN = 100
	if _, ok := plain.AcceptanceLength(); ok {
		t.Fatal("a server that reported no drafts must not report an acceptance length")
	}

	none := 0
	off := &Response{}
	off.Timings.PredictedN = 100
	off.Timings.DraftN, off.Timings.DraftNAccepted = &none, &none
	if _, ok := off.AcceptanceLength(); ok {
		t.Fatal("drafting nothing is not an acceptance measurement")
	}
}

func TestDecodeSecondsNeedsAStream(t *testing.T) {
	unstreamed := &Response{Wall: 10 * time.Second}
	if _, ok := unstreamed.DecodeSeconds(); ok {
		t.Fatal("decode cannot be isolated without a stream")
	}
	streamed := &Response{Wall: 10 * time.Second, TTFT: 4 * time.Second, Streamed: true}
	secs, ok := streamed.DecodeSeconds()
	if !ok || secs != 6 {
		t.Fatalf("decode = %v, %v; want 6, true", secs, ok)
	}
}

// The whole point of streaming here is the boundary between prefill and decode, so the
// test holds the first token back and checks the gap lands on the right side of it.
func TestStreamMeasuresDecodeApartFromPrefill(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flush, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("the test server must flush, or there is no stream to measure")
		}
		time.Sleep(120 * time.Millisecond) // prefill
		for _, tok := range []string{"O", "K"} {
			_, _ = fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", tok)
			flush.Flush()
			time.Sleep(10 * time.Millisecond)
		}
		_, _ = fmt.Fprint(w, `data: {"choices":[{"finish_reason":"stop","delta":{}}],`+
			`"usage":{"completion_tokens":2,"prompt_tokens":7},`+
			`"timings":{"predicted_n":2,"draft_n":3,"draft_n_accepted":1}}`+"\n\n")
		flush.Flush()
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 10*time.Second)
	c.Stream = true
	resp, err := c.Complete(context.Background(), chatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 8, Stream: true})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if got := resp.Choices[0].Message.Content; got != "OK" {
		t.Fatalf("content = %q, want %q", got, "OK")
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Fatalf("finish reason = %q", resp.Choices[0].FinishReason)
	}
	if resp.Usage.CompletionTokens != 2 {
		t.Fatalf("usage did not survive the final chunk: %+v", resp.Usage)
	}
	if resp.TTFT < 100*time.Millisecond {
		t.Fatalf("ttft = %v, want at least the prefill this server slept", resp.TTFT)
	}
	secs, ok := resp.DecodeSeconds()
	if !ok {
		t.Fatal("a streamed reply must yield a decode time")
	}
	if secs >= resp.Wall.Seconds() {
		t.Fatalf("decode %.3fs is not shorter than wall %.3fs — prefill was not excluded",
			secs, resp.Wall.Seconds())
	}
	if _, ok := resp.AcceptanceLength(); !ok {
		t.Fatal("draft counters on the final chunk must reach the acceptance figure")
	}
}

// Tools are scored from the reply's shape, and streamed tool calls arrive as fragments
// this client does not reassemble. Run must therefore leave those tasks unstreamed —
// a speed number is not worth a scoring bug.
func TestRunWillNotStreamATaskCarryingTools(t *testing.T) {
	var streamed []bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Stream bool `json:"stream"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		streamed = append(streamed, req.Stream)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"choices":[{"finish_reason":"stop","message":{"content":"x"}}]}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 5*time.Second)
	c.Stream = true
	withTools := &Task{ID: "t", Kind: "toolcall", MaxTokens: 8,
		Messages: []Message{{Role: "user", Content: "hi"}},
		Tools:    []Tool{{Type: "function", Function: ToolFunction{Name: "f"}}}}
	if _, err := c.Run(context.Background(), withTools, Sampling{}, nil, "", nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(streamed) != 1 || streamed[0] {
		t.Fatalf("a task with tools was streamed: %v", streamed)
	}
}
