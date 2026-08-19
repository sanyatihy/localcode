package eval

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func messagesServer(t *testing.T, reply string, seen *messagesRequest) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("posted to %s, want /v1/messages", r.URL.Path)
		}
		if got := r.Header.Get("anthropic-version"); got != anthropicVersion {
			t.Errorf("anthropic-version %q, want %q", got, anthropicVersion)
		}
		if seen != nil {
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, seen); err != nil {
				t.Errorf("request body: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, reply)
	}))
}

// The conversion is the thing under test: a fixture written for chat-completions has to
// arrive as a Messages request without losing the system prompt or the tool schema.
func TestMessagesRequestCarriesSystemAndTools(t *testing.T) {
	var seen messagesRequest
	srv := messagesServer(t, `{"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn"}`, &seen)
	defer srv.Close()

	c := NewClient(srv.URL, 5*time.Second)
	c.API = APIMessages
	_, err := c.Complete(context.Background(), chatRequest{
		Messages: []Message{
			{Role: "system", Content: "be terse"},
			{Role: "user", Content: "hello"},
		},
		Tools: []Tool{{Type: "function", Function: ToolFunction{
			Name: "edit_file", Description: "edit it",
			Parameters: map[string]any{"type": "object"},
		}}},
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if seen.System != "be terse" {
		t.Errorf("system = %q, want it beside the conversation rather than in it", seen.System)
	}
	if len(seen.Messages) != 1 || seen.Messages[0].Role != "user" {
		t.Errorf("messages = %+v, want the user turn alone", seen.Messages)
	}
	if len(seen.Tools) != 1 || seen.Tools[0].Name != "edit_file" || seen.Tools[0].InputSchema == nil {
		t.Errorf("tools = %+v, want the schema under input_schema", seen.Tools)
	}
	if seen.MaxTokens != 64 {
		t.Errorf("max_tokens = %d, want 64", seen.MaxTokens)
	}
}

// A tool call has to come back in the shape checkToolCall reads, or the two paths are
// scored by different rules and their numbers cannot be compared.
func TestMessagesToolUseIsScoredLikeAToolCall(t *testing.T) {
	srv := messagesServer(t, `{"content":[
		{"type":"thinking","thinking":"hmm"},
		{"type":"tool_use","id":"tu_1","name":"propose_patch","input":{"path":"session.go","diff":"-a\n+b"}}
	],"stop_reason":"tool_use","usage":{"input_tokens":4,"output_tokens":9,"cache_read_input_tokens":19}}`, nil)
	defer srv.Close()

	c := NewClient(srv.URL, 5*time.Second)
	c.API = APIMessages
	resp, err := c.Complete(context.Background(), chatRequest{
		Messages: []Message{{Role: "user", Content: "fix it"}}, MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	msg := resp.Choices[0].Message
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Function.Name != "propose_patch" {
		t.Fatalf("tool calls = %+v", msg.ToolCalls)
	}
	outcome, detail := checkToolCall(Expect{
		Tool: "propose_patch", RequiredArgs: []string{"path", "diff"},
		ArgContains: map[string]string{"path": "session.go"},
	}, msg.ToolCalls, msg.Content)
	if outcome != Pass {
		t.Errorf("outcome = %s (%s), want pass", outcome, detail)
	}
	if msg.ReasoningContent != "hmm" {
		t.Errorf("reasoning = %q, want it kept out of the answer", msg.ReasoningContent)
	}
	// Cached tokens are excluded from input_tokens here and included in prompt_tokens on
	// the chat path; a prompt that shrinks as the cache warms is the bug this guards.
	if resp.Usage.PromptTokens != 23 {
		t.Errorf("prompt tokens = %d, want 4+19", resp.Usage.PromptTokens)
	}
}

func TestMessagesTruncationIsNotAQualityFailure(t *testing.T) {
	srv := messagesServer(t, `{"content":[{"type":"text","text":"partial"}],"stop_reason":"max_tokens"}`, nil)
	defer srv.Close()

	c := NewClient(srv.URL, 5*time.Second)
	c.API = APIMessages
	resp, err := c.Complete(context.Background(), chatRequest{
		Messages: []Message{{Role: "user", Content: "x"}}, MaxTokens: 8,
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if resp.Choices[0].FinishReason != "length" {
		t.Errorf("finish reason = %q, want the string Run checks for truncation",
			resp.Choices[0].FinishReason)
	}
}

// Sampling and the thinking toggle have no place in a Messages body. Dropping them
// silently would measure the server's defaults while the row claimed the flags.
func TestMessagesRefusesSamplingItCannotSend(t *testing.T) {
	srv := messagesServer(t, `{"content":[{"type":"text","text":"ok"}]}`, nil)
	defer srv.Close()
	c := NewClient(srv.URL, 5*time.Second)
	c.API = APIMessages

	temp := 0.7
	for _, req := range []chatRequest{
		{Messages: []Message{{Role: "user", Content: "x"}}, Sampling: Sampling{Temperature: &temp}},
		{Messages: []Message{{Role: "user", Content: "x"}}, ChatTemplateKwargs: map[string]any{"enable_thinking": false}},
		{Messages: []Message{{Role: "user", Content: "x"}}, ReasoningEffort: "low"},
	} {
		if _, err := c.Complete(context.Background(), req); !errors.Is(err, errSamplingNotSendable) {
			t.Errorf("got %v, want a refusal naming the server as the place sampling lives", err)
		}
	}
}
