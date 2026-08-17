// Package eval talks to an OpenAI-compatible endpoint and scores fixed tasks
// against it. It deliberately speaks HTTP directly rather than driving an agent
// harness: tier-1 tasks are single-turn, and a harness would add its own prompt,
// its own retries and its own token overhead to every measurement.
package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Sampling is the request-level toggle set — the axis 0005 sweeps. Pointers so an
// unset field is omitted and the server's own default applies, which keeps "we did
// not set this" distinguishable from "we set it to zero".
type Sampling struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"top_p,omitempty"`
	TopK            *int     `json:"top_k,omitempty"`
	MinP            *float64 `json:"min_p,omitempty"`
	PresencePenalty *float64 `json:"presence_penalty,omitempty"`
}

type chatRequest struct {
	Messages           []Message      `json:"messages"`
	Tools              []Tool         `json:"tools,omitempty"`
	ToolChoice         string         `json:"tool_choice,omitempty"`
	MaxTokens          int            `json:"max_tokens"`
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
	Sampling
}

type ToolCall struct {
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type Response struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content          string     `json:"content"`
			ReasoningContent string     `json:"reasoning_content"`
			ToolCalls        []ToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Timings struct {
		PromptN            int     `json:"prompt_n"`
		PromptPerSecond    float64 `json:"prompt_per_second"`
		PredictedN         int     `json:"predicted_n"`
		PredictedPerSecond float64 `json:"predicted_per_second"`
	} `json:"timings"`
	Usage struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
	Error json.RawMessage `json:"error"`

	Wall time.Duration `json:"-"` // measured here, not reported by the server
}

type Client struct {
	Endpoint string
	HTTP     *http.Client
}

func NewClient(endpoint string, timeout time.Duration) *Client {
	return &Client{Endpoint: endpoint, HTTP: &http.Client{Timeout: timeout}}
}

// Complete sends one chat completion. A non-nil error means the request itself
// failed; a server-reported error arrives in Response.Error so the caller can
// record it as a task failure rather than a harness failure — the distinction
// matters when a config is rejected for exceeding context.
func (c *Client) Complete(ctx context.Context, req chatRequest) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.Endpoint+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response (http %d): %w", resp.StatusCode, err)
	}
	out.Wall = time.Since(start)
	return &out, nil
}
