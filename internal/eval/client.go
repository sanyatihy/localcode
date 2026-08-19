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
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
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
	Messages   []Message `json:"messages"`
	Tools      []Tool    `json:"tools,omitempty"`
	ToolChoice string    `json:"tool_choice,omitempty"`
	MaxTokens  int       `json:"max_tokens"`

	// ReasoningEffort is xhigh, medium or low, and it is a separate axis from
	// enable_thinking rather than a finer version of it: thinking off is off, and
	// with thinking on this decides how much of it there is. Qwen3.8 defaults to
	// xhigh, which its own release notes describe as being for "complex tasks
	// demanding thorough analysis" and which does not terminate on some of ours.
	// Empty omits the field and leaves that default in force — recorded per run,
	// because a thinking number without it does not say what was measured.
	ReasoningEffort    string         `json:"reasoning_effort,omitempty"`
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

	// API selects the dialect: APIChat is chat-completions, APIMessages is the
	// Anthropic Messages path llama-server converts internally. Empty means APIChat,
	// so every existing caller keeps the path its numbers were taken on.
	API string
}

func NewClient(endpoint string, timeout time.Duration) *Client {
	return &Client{Endpoint: endpoint, HTTP: &http.Client{Timeout: timeout}}
}

// Complete sends one chat completion. A non-nil error means the request itself
// failed; a server-reported error arrives in Response.Error so the caller can
// record it as a task failure rather than a harness failure — the distinction
// matters when a config is rejected for exceeding context.
func (c *Client) Complete(ctx context.Context, req chatRequest) (*Response, error) {
	if c.API == APIMessages {
		return c.completeMessages(ctx, req)
	}
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
	defer func() { _ = resp.Body.Close() }()

	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response (http %d): %w", resp.StatusCode, err)
	}
	out.Wall = time.Since(start)
	return &out, nil
}

// ServerProps is what the endpoint reports about itself. Recording it beside every
// result is a guard against the sweep's most damaging silent failure: mislabelling.
// When a run varies server-level flags, a config label is a human's claim about what
// was launched, and a row that carries the served n_ctx cannot quietly attribute one
// config's numbers to another.
// ServerProps is what a backend says it is serving. Not every backend says: llama.cpp
// exposes /props, an MLX server may expose nothing, and a row must be able to record
// "unavailable" rather than a confident zero that reads as "0 context".
type ServerProps struct {
	NCtx      int    `json:"n_ctx"`
	ModelPath string `json:"model_path"`

	// Available is false when the backend could not be asked. The scorer still runs —
	// a backend that cannot introspect is scoreable, it just cannot have its served
	// config checked against the label a human typed.
	Available bool `json:"available"`
}

// ServerMetrics is what the endpoint has counted since it started. A harness builds its
// own requests and none of the four accounts for them in the same units, so what a run
// cost is only comparable at the server: sampled either side of a run, the difference is
// that run's.
//
// Prompt tokens are split the way llama.cpp splits them. Processed tokens were ingested;
// cached ones were reused from a prefix the server still held. A harness that keeps a
// stable prefix across turns pays the second, and one that rewrites its history pays the
// first — at this depth that is minutes, and it is the difference the comparison is for.
type ServerMetrics struct {
	PromptTokens    int // processed, not served from cache
	CachedTokens    int
	PredictedTokens int
	Available       bool
}

// Sub returns the metrics accumulated between two samples. An unavailable endpoint on
// either side leaves the result unavailable rather than confidently zero.
func (m ServerMetrics) Sub(earlier ServerMetrics) ServerMetrics {
	if !m.Available || !earlier.Available {
		return ServerMetrics{}
	}
	return ServerMetrics{
		PromptTokens:    m.PromptTokens - earlier.PromptTokens,
		CachedTokens:    m.CachedTokens - earlier.CachedTokens,
		PredictedTokens: m.PredictedTokens - earlier.PredictedTokens,
		Available:       true,
	}
}

// Metrics reads llama.cpp's Prometheus counters. The endpoint answers 501 unless the
// server was started with --metrics, which is a configuration fact rather than a failure:
// the caller records the run without token counts and says so.
func (c *Client) Metrics(ctx context.Context) (ServerMetrics, error) {
	var out ServerMetrics
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Endpoint+"/metrics", nil)
	if err != nil {
		return out, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return out, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("%s/metrics: %s", c.Endpoint, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return out, err
	}
	return parseMetrics(string(body))
}

// parseMetrics reads the Prometheus text format, which is one "name value" per line with
// comments starting #. Only the three counters this project uses are pulled out, and a
// missing one is an error: a zero would read as a run that cost nothing.
func parseMetrics(body string) (ServerMetrics, error) {
	want := map[string]*int{}
	var out ServerMetrics
	want["llamacpp:prompt_tokens_total"] = &out.PromptTokens
	want["llamacpp:prompt_tokens_cached_total"] = &out.CachedTokens
	want["llamacpp:tokens_predicted_total"] = &out.PredictedTokens

	seen := 0
	for _, line := range strings.Split(body, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		field, wanted := want[name]
		if !wanted {
			continue
		}
		// Counters are exported as floats, and a token count is whole either way.
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return ServerMetrics{}, fmt.Errorf("%s: %w", name, err)
		}
		*field = int(f)
		seen++
	}
	if seen != len(want) {
		return ServerMetrics{}, fmt.Errorf("metrics carried %d of the %d counters this needs", seen, len(want))
	}
	out.Available = true
	return out, nil
}

// TurnCounter counts a harness's turns while it works, by watching which task the
// server's slot is busy with. Each chat completion occupies the slot under a task id of
// its own, so the number of distinct ids seen busy is the number of requests the harness
// made — the same instrument for every harness, where each harness's own accounting is
// in units of its own.
//
// It samples rather than intercepts, so a request that starts and finishes inside one
// interval is missed. At the depths this project serves, a turn costs seconds of ingest
// alone; the interval is recorded with the count so the assumption is visible.
type TurnCounter struct {
	mu    sync.Mutex
	seen  map[int]bool
	stop  chan struct{}
	ended chan struct{}
}

// CountTurns starts watching. Stop returns what it saw.
func (c *Client) CountTurns(ctx context.Context, every time.Duration) *TurnCounter {
	t := &TurnCounter{seen: map[int]bool{}, stop: make(chan struct{}), ended: make(chan struct{})}
	go func() {
		defer close(t.ended)
		tick := time.NewTicker(every)
		defer tick.Stop()
		for {
			select {
			case <-t.stop:
				return
			case <-ctx.Done():
				return
			case <-tick.C:
				busy, err := c.busyTasks(ctx)
				if err != nil {
					continue // a sample that failed is one sample, not a broken run
				}
				t.mu.Lock()
				for _, id := range busy {
					t.seen[id] = true
				}
				t.mu.Unlock()
			}
		}
	}()
	return t
}

// Stop ends the watch and returns how many distinct tasks the slot was seen working on.
func (t *TurnCounter) Stop() int {
	close(t.stop)
	<-t.ended
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.seen)
}

func (c *Client) busyTasks(ctx context.Context) ([]int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Endpoint+"/slots", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var slots []struct {
		IDTask       int  `json:"id_task"`
		IsProcessing bool `json:"is_processing"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&slots); err != nil {
		return nil, err
	}
	var out []int
	for _, s := range slots {
		if s.IsProcessing {
			out = append(out, s.IDTask)
		}
	}
	return out, nil
}

func (c *Client) Props(ctx context.Context) (ServerProps, error) {
	var out ServerProps
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Endpoint+"/props", nil)
	if err != nil {
		return out, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return out, err
	}
	defer func() { _ = resp.Body.Close() }()

	var raw struct {
		ModelPath string `json:"model_path"`
		Gen       struct {
			NCtx int `json:"n_ctx"`
		} `json:"default_generation_settings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return out, err
	}
	return ServerProps{NCtx: raw.Gen.NCtx, ModelPath: raw.ModelPath, Available: true}, nil
}
