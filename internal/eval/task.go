package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultTimeoutSeconds bounds a task that does not set its own budget. Generous
// enough for the slowest fixture measured at the fast end, tight enough that a stuck
// task costs minutes rather than the afternoon.
const DefaultTimeoutSeconds = 120

// Task is one tier-1 fixture: a single-turn request plus a deterministic check.
// "Deterministic" is the whole point — an LLM judge would add a second model's
// noise to every number this project rests on.
type Task struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"` // toolcall | patch | retrieval
	Messages  []Message `json:"messages"`
	Tools     []Tool    `json:"tools,omitempty"`
	MaxTokens int       `json:"max_tokens"`

	// TimeoutSeconds bounds one attempt. A task with no ceiling is not a measurement,
	// it is a hostage: this suite has produced 15-minute single runs that ended in no
	// answer at all. Over-budget is scored as its own outcome so it is never mistaken
	// for a wrong answer, and the number lives in the fixture because how long a task
	// may reasonably take is a property of the task.
	TimeoutSeconds int `json:"timeout_seconds"`

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

	// FailTruncated is not a quality failure. It means the answer was cut off at
	// max_tokens, which measures the budget the fixture granted rather than anything
	// about the model. Kept distinct because thinking mode spends the same budget on
	// reasoning first, so a shared cap silently penalises it.
	FailTruncated Outcome = "fail_truncated_at_cap"

	// FailOverBudget is likewise not a quality failure: the model was still working
	// when its clock ran out. It is the outcome that keeps a sweep bounded, and a
	// suite where it appears often is badly budgeted rather than badly answered.
	FailOverBudget Outcome = "fail_over_budget"
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
	if t.TimeoutSeconds == 0 {
		t.TimeoutSeconds = DefaultTimeoutSeconds
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
// as setting it false. effort is the reasoning_effort level; empty leaves the
// model's default, which for Qwen3.8 is xhigh.
func (c *Client) Run(ctx context.Context, t *Task, s Sampling, thinking *bool, effort string) (Result, error) {
	msgs, err := t.expand()
	if err != nil {
		return Result{TaskID: t.ID, Outcome: FailServer, Detail: err.Error()}, err
	}
	req := chatRequest{
		Messages:        msgs,
		Tools:           t.Tools,
		MaxTokens:       t.MaxTokens,
		ReasoningEffort: effort,
		Sampling:        s,
	}
	if len(t.Tools) > 0 {
		req.ToolChoice = "auto"
	}
	if thinking != nil {
		req.ChatTemplateKwargs = map[string]any{"enable_thinking": *thinking}
	}

	// Defaulted here as well as in LoadTask: Run must not assume its caller came
	// through the loader, and a zero budget meaning "expire immediately" would turn a
	// hand-built Task into a suite of instant failures.
	budget := t.TimeoutSeconds
	if budget <= 0 {
		budget = DefaultTimeoutSeconds
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(budget)*time.Second)
	defer cancel()

	resp, err := c.Complete(runCtx, req)
	if err != nil {
		// The task's own clock expiring is a scored outcome, not a transport failure,
		// and must not abort the suite: returning the error here would end the run.
		if runCtx.Err() != nil && ctx.Err() == nil {
			return Result{TaskID: t.ID, Outcome: FailOverBudget,
				Detail: fmt.Sprintf("exceeded its %ds budget", budget)}, nil
		}
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

	// Checked before the per-kind check: a truncated reply can fail any of them for a
	// reason that is not the model's, and attributing it to quality would be wrong.
	if resp.Choices[0].FinishReason == "length" {
		res.Outcome = FailTruncated
		res.Detail = fmt.Sprintf("hit max_tokens=%d after %d reasoning chars",
			t.MaxTokens, res.ReasoningChars)
		return res, nil
	}

	switch t.Kind {
	case "toolcall":
		res.Outcome, res.Detail = checkToolCall(t.Expect, msg.ToolCalls, msg.Content)
	case "patch":
		res.Outcome, res.Detail = runPatch(ctx, *t.Patch, extractCode(msg.Content))
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
