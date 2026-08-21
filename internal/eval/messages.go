package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// The same tasks and the same grader, sent down the Anthropic Messages path instead of
// chat-completions — the path a Claude Code session takes, and a conversion inside
// llama-server. Everything here is one request shape translated to another and back; the
// scoring, the outcomes and the row are untouched, which is what makes the two sets of
// numbers comparable.

// APIChat and APIMessages name the dialect a Client speaks.
const (
	APIChat     = "chat"
	APIMessages = "messages"
)

// anthropicVersion is required on every request to the Messages API and is a constant of
// the protocol rather than a choice.
const anthropicVersion = "2023-06-01"

type messagesRequest struct {
	Model     string          `json:"model"`
	System    string          `json:"system,omitempty"`
	Messages  []Message       `json:"messages"`
	Tools     []anthropicTool `json:"tools,omitempty"`
	MaxTokens int             `json:"max_tokens"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type messagesResponse struct {
	Content []struct {
		Type     string          `json:"type"`
		Text     string          `json:"text"`
		Thinking string          `json:"thinking"`
		Name     string          `json:"name"`
		Input    json.RawMessage `json:"input"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens           int `json:"input_tokens"`
		OutputTokens          int `json:"output_tokens"`
		CacheReadInputTokens  int `json:"cache_read_input_tokens"`
		CacheWriteInputTokens int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
	Error json.RawMessage `json:"error"`
}

// errSamplingNotSendable is returned rather than silently dropping a flag: the Messages
// API carries no presence_penalty and no chat_template_kwargs, so a run asking for them
// would be measured at the server's defaults while its row claimed otherwise. On this path
// sampling and the toggle are served, which is what a Claude Code session gets too.
var errSamplingNotSendable = errors.New(
	"the messages path takes sampling and the thinking toggle from the server: serve config/agent.env and pass neither")

// completeMessages sends one request down the Anthropic Messages path and returns it in
// the same shape the chat path produces, so Run scores both identically.
func (c *Client) completeMessages(ctx context.Context, req chatRequest) (*Response, error) {
	if req.Sampling != (Sampling{}) || req.ChatTemplateKwargs != nil || req.ReasoningEffort != "" {
		return nil, errSamplingNotSendable
	}

	var out messagesRequest
	out.MaxTokens = req.MaxTokens
	// Required by the protocol and ignored by llama-server, which serves whatever it
	// loaded. What was actually served is read from /props and recorded on the row, so
	// nothing rests on this string.
	out.Model = "local"
	for _, m := range req.Messages {
		if m.Role == "system" {
			// This API carries the system prompt beside the conversation rather than
			// in it. Joining is safe because a fixture carries at most one.
			out.System = strings.TrimSpace(out.System + "\n\n" + m.Content)
			continue
		}
		out.Messages = append(out.Messages, m)
	}
	for _, t := range req.Tools {
		out.Tools = append(out.Tools, anthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}

	body, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.Endpoint+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	start := time.Now()
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var raw messagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode response (http %d): %w", resp.StatusCode, err)
	}
	return raw.toResponse(time.Since(start)), nil
}

// toResponse maps a Messages reply onto the chat-completions shape the scorer reads.
// Server-side timings have no equivalent here and stay zero, which the row already
// treats as "not reported" rather than as zero throughput.
func (r *messagesResponse) toResponse(wall time.Duration) *Response {
	var out Response
	out.Wall = wall
	out.Error = r.Error

	var text, thinking strings.Builder
	var calls []ToolCall
	for _, b := range r.Content {
		switch b.Type {
		case "text":
			text.WriteString(b.Text)
		case "thinking":
			thinking.WriteString(b.Thinking)
		case "tool_use":
			var tc ToolCall
			tc.Function.Name = b.Name
			// Arguments is a JSON string on the chat path and an object here. Keeping
			// the string form means checkToolCall's invalid-JSON outcome still means
			// what it says: the model emitted something unparseable.
			tc.Function.Arguments = string(b.Input)
			calls = append(calls, tc)
		}
	}

	var choice Choice
	// "length" is the string Run checks for a truncated answer; stop_reason is the
	// Messages API's name for the same thing.
	if r.StopReason == "max_tokens" {
		choice.FinishReason = "length"
	} else {
		choice.FinishReason = r.StopReason
	}
	choice.Message.Content = text.String()
	choice.Message.ReasoningContent = thinking.String()
	choice.Message.ToolCalls = calls

	// A reply with neither content nor a tool call and no error is not something the
	// scorer can grade; leaving Choices empty makes Run record it as a server failure
	// rather than as prose.
	if len(r.Content) > 0 || len(r.Error) > 0 {
		out.Choices = append(out.Choices, choice)
	}

	// input_tokens on this path counts only what was not served from cache, while
	// prompt_tokens on the chat path counts the whole prompt. Summing them is what makes
	// the two comparable; recording input_tokens raw would show a prompt shrinking as the
	// cache warmed.
	out.Usage.PromptTokens = r.Usage.InputTokens + r.Usage.CacheReadInputTokens
	out.Usage.CompletionTokens = r.Usage.OutputTokens
	out.Usage.PromptTokensDetails.CachedTokens = r.Usage.CacheReadInputTokens
	return &out
}
