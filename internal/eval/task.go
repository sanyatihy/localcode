package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Task is one tier-1 fixture: a single-turn request plus a deterministic check.
// "Deterministic" is the whole point — an LLM judge would add a second model's
// noise to every number this project rests on.
type Task struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"` // toolcall | patch | retrieval
	Messages  []Message `json:"messages"`
	Tools     []Tool    `json:"tools,omitempty"`
	MaxTokens int       `json:"max_tokens"`

	Expect    Expect     `json:"expect"`
	Patch     *Patch     `json:"patch,omitempty"`
	Retrieval *Retrieval `json:"retrieval,omitempty"`
}

type Expect struct {
	Tool         string            `json:"tool"`          // tool name that must be called
	RequiredArgs []string          `json:"required_args"` // keys that must be present in arguments
	ArgContains  map[string]string `json:"arg_contains"`  // key -> substring its value must contain
}

// Outcome separates the ways a task can fail, because they mean different things.
// A wrong answer is the model's; a malformed tool call is the format problem 0005
// exists to chase; a server error is neither and must not be scored as quality.
type Outcome string

const (
	Pass          Outcome = "pass"
	FailNoCall    Outcome = "fail_no_tool_call"
	FailWrongTool Outcome = "fail_wrong_tool"
	FailBadJSON   Outcome = "fail_invalid_json"
	FailArgs      Outcome = "fail_wrong_args"
	FailServer    Outcome = "fail_server_error"

	FailEmpty     Outcome = "fail_empty_answer"
	FailCompile   Outcome = "fail_does_not_compile"
	FailTest      Outcome = "fail_test_failed"
	FailRetrieval Outcome = "fail_sentinel_not_recalled"
)

type Result struct {
	TaskID  string  `json:"task_id"`
	Outcome Outcome `json:"outcome"`
	Detail  string  `json:"detail,omitempty"`

	PromptTokens     int     `json:"prompt_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	ReasoningChars   int     `json:"reasoning_chars"`
	PromptPerSecond  float64 `json:"prompt_per_second"`
	GenPerSecond     float64 `json:"gen_per_second"`
	WallSeconds      float64 `json:"wall_seconds"`
}

func (r Result) Passed() bool { return r.Outcome == Pass }

func LoadTask(path string) (*Task, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Task
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if t.ID == "" || t.Kind == "" {
		return nil, fmt.Errorf("%s: task needs an id and a kind", path)
	}
	if t.MaxTokens == 0 {
		t.MaxTokens = 512
	}
	switch t.Kind {
	case "toolcall":
	case "patch":
		if t.Patch == nil {
			return nil, fmt.Errorf("%s: patch task needs a patch block", path)
		}
		t.Patch.Dir = filepath.Dir(path)
	case "retrieval":
		if t.Retrieval == nil {
			return nil, fmt.Errorf("%s: retrieval task needs a retrieval block", path)
		}
	default:
		return nil, fmt.Errorf("%s: unknown kind %q", path, t.Kind)
	}
	return &t, nil
}

// expand substitutes the generated body of a task into its prompt. Patch and
// retrieval tasks are templates: the fixture holds the instruction, and the bulk
// of the prompt is built here so it is identical across configs.
func (t *Task) expand() ([]Message, error) {
	var body string
	switch t.Kind {
	case "patch":
		src, err := os.ReadFile(filepath.Join(t.Patch.Dir, t.Patch.Source))
		if err != nil {
			return nil, fmt.Errorf("fixture source unreadable: %w", err)
		}
		body = string(src)
	case "retrieval":
		body = buildHaystack(*t.Retrieval)
	default:
		return t.Messages, nil
	}
	out := make([]Message, len(t.Messages))
	copy(out, t.Messages)
	for i := range out {
		out[i].Content = strings.ReplaceAll(out[i].Content, "{{BODY}}", body)
	}
	return out, nil
}

// Run executes one task and scores it. thinking is passed through to the model's
// own chat template; nil leaves the template default alone, which is not the same
// as setting it false.
func (c *Client) Run(ctx context.Context, t *Task, s Sampling, thinking *bool) (Result, error) {
	msgs, err := t.expand()
	if err != nil {
		return Result{TaskID: t.ID, Outcome: FailServer, Detail: err.Error()}, err
	}
	req := chatRequest{
		Messages:  msgs,
		Tools:     t.Tools,
		MaxTokens: t.MaxTokens,
		Sampling:  s,
	}
	if len(t.Tools) > 0 {
		req.ToolChoice = "auto"
	}
	if thinking != nil {
		req.ChatTemplateKwargs = map[string]any{"enable_thinking": *thinking}
	}

	resp, err := c.Complete(ctx, req)
	if err != nil {
		return Result{TaskID: t.ID, Outcome: FailServer, Detail: err.Error()}, err
	}

	res := Result{
		TaskID:           t.ID,
		PromptTokens:     resp.Usage.PromptTokens,
		CachedTokens:     resp.Usage.PromptTokensDetails.CachedTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		PromptPerSecond:  resp.Timings.PromptPerSecond,
		GenPerSecond:     resp.Timings.PredictedPerSecond,
		WallSeconds:      resp.Wall.Seconds(),
	}
	if len(resp.Error) > 0 && string(resp.Error) != "null" {
		res.Outcome, res.Detail = FailServer, truncate(string(resp.Error), 200)
		return res, nil
	}
	if len(resp.Choices) == 0 {
		res.Outcome, res.Detail = FailServer, "no choices returned"
		return res, nil
	}
	msg := resp.Choices[0].Message
	res.ReasoningChars = len(msg.ReasoningContent)

	switch t.Kind {
	case "toolcall":
		res.Outcome, res.Detail = checkToolCall(t.Expect, msg.ToolCalls, msg.Content)
	case "patch":
		res.Outcome, res.Detail = runPatch(*t.Patch, extractCode(msg.Content))
	case "retrieval":
		res.Outcome, res.Detail = checkRetrieval(*t.Retrieval, msg.Content)
	default:
		res.Outcome, res.Detail = FailServer, "unknown task kind "+t.Kind
	}
	return res, nil
}

func checkToolCall(exp Expect, calls []ToolCall, content string) (Outcome, string) {
	if len(calls) == 0 {
		return FailNoCall, "answered in prose: " + truncate(strings.TrimSpace(content), 120)
	}
	got := calls[0].Function
	if got.Name != exp.Tool {
		return FailWrongTool, fmt.Sprintf("called %q, wanted %q", got.Name, exp.Tool)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(got.Arguments), &args); err != nil {
		return FailBadJSON, fmt.Sprintf("%v in %s", err, truncate(got.Arguments, 120))
	}
	for _, k := range exp.RequiredArgs {
		if _, ok := args[k]; !ok {
			return FailArgs, fmt.Sprintf("missing required arg %q in %s", k, got.Arguments)
		}
	}
	for k, want := range exp.ArgContains {
		v, ok := args[k].(string)
		if !ok {
			return FailArgs, fmt.Sprintf("arg %q is not a string: %s", k, got.Arguments)
		}
		if !strings.Contains(v, want) {
			return FailArgs, fmt.Sprintf("arg %q = %q, wanted it to contain %q", k, v, want)
		}
	}
	return Pass, ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
