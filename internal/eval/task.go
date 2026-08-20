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

	// Tier2 marks a fixture as drivable by a harness as well as answerable in one
	// request, and carries the one thing tier 2 needs and tier 1 does not: the name the
	// broken file takes in a scratch checkout. What the bug is stays in Messages, so a
	// fixture cannot pose one problem to a harness and a different one to the model.
	Tier2 *Tier2Spec `json:"tier2,omitempty"`
}

// Tier2Spec is a patch fixture's tier-2 half.
type Tier2Spec struct {
	AnswerName string `json:"answer_name"`
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

	// Inadmissible is not a result about the harness at all: it never ran, because the
	// context it requires is one the machine cannot serve under the profile being
	// scored. It is a row rather than an omission so that a comparison table says why
	// a harness is absent — a gap where a number should be is read as an oversight,
	// and the reader cannot tell a constraint from a run somebody forgot.
	Inadmissible Outcome = "inadmissible"
)

type Result struct {
	TaskID  string  `json:"task_id"`
	Outcome Outcome `json:"outcome"`
	Detail  string  `json:"detail,omitempty"`

	// Memory around the run. A run whose swap grew was measuring the pager, and the
	// vision calls that void rather than slow — so it is recorded per row and the
	// reporter flags it rather than averaging it in.
	FreeGB      float64 `json:"free_gb"`
	SwapDeltaMB float64 `json:"swap_delta_mb"`
	MemMeasured bool    `json:"mem_measured"`

	PromptTokens     int     `json:"prompt_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	ReasoningChars   int     `json:"reasoning_chars"`
	PromptPerSecond  float64 `json:"prompt_per_second"`
	GenPerSecond     float64 `json:"gen_per_second"`
	WallSeconds      float64 `json:"wall_seconds"`

	// Decode, separated from prefill by the client rather than by the server. A
	// speculative decoder moves decode and cannot move prefill, so wall hides the whole
	// effect on any run whose prompt is deep — the editor profile is 96.7% prefill.
	// DecodeMeasured is false when the run was not streamed, and the rate is then absent
	// rather than zero.
	TTFTSeconds     float64 `json:"ttft_seconds,omitempty"`
	DecodeSeconds   float64 `json:"decode_seconds,omitempty"`
	DecodePerSecond float64 `json:"decode_per_second,omitempty"`
	DecodeMeasured  bool    `json:"decode_measured"`

	// Acceptance length, and whether it could be read at all. A server that does not
	// speculate reports nothing here, which is not an acceptance of zero.
	AcceptanceLength   float64 `json:"acceptance_length,omitempty"`
	AcceptanceMeasured bool    `json:"acceptance_measured"`
	DraftN             int     `json:"draft_n,omitempty"`
	DraftAccepted      int     `json:"draft_accepted,omitempty"`
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
func (c *Client) Run(ctx context.Context, t *Task, s Sampling, thinking *bool, effort string, prof *Profile) (Result, error) {
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
	// Streamed only when nothing else depends on the reply's shape. Tool calls arrive as
	// fragments that would have to be reassembled to be scored, and a speed measurement
	// is not worth a scoring bug: those tasks keep the unstreamed path and their rows say
	// decode was not measured.
	req.Stream = c.Stream && len(t.Tools) == 0
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
	started := time.Now()
	before := sampleMemory()

	resp, err := c.Complete(runCtx, req)
	if err != nil {
		// The task's own clock expiring is a scored outcome, not a transport failure,
		// and must not abort the suite: returning the error here would end the run.
		if runCtx.Err() != nil && ctx.Err() == nil {
			// Carry the wall clock. An over-budget run is precisely the one whose
			// duration is the finding, and recording zero both loses it and understates
			// every total the row is summed into.
			return Result{TaskID: t.ID, Outcome: FailOverBudget,
				WallSeconds: time.Since(started).Seconds(),
				Detail:      fmt.Sprintf("exceeded its %ds budget", budget)}, nil
		}
		// A request went out and took time even though it failed, so recording zero
		// would understate any total this row is summed into.
		return Result{TaskID: t.ID, Outcome: FailServer, Detail: err.Error(),
			WallSeconds: time.Since(started).Seconds()}, err
	}

	after := sampleMemory()
	// Wall is measured here, client side, on purpose: it is the only speed number every
	// backend can produce. Server-reported tok/s exists on llama.cpp and may not exist
	// elsewhere, so it is recorded where available and never used to compare backends.
	res := Result{
		TaskID:           t.ID,
		FreeGB:           after.FreeGB,
		SwapDeltaMB:      after.SwapUsedMB - before.SwapUsedMB,
		MemMeasured:      before.OK && after.OK,
		PromptTokens:     resp.Usage.PromptTokens,
		CachedTokens:     resp.Usage.PromptTokensDetails.CachedTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		PromptPerSecond:  resp.Timings.PromptPerSecond,
		GenPerSecond:     resp.Timings.PredictedPerSecond,
		WallSeconds:      resp.Wall.Seconds(),
	}
	if secs, ok := resp.DecodeSeconds(); ok {
		res.TTFTSeconds = resp.TTFT.Seconds()
		res.DecodeSeconds = secs
		res.DecodeMeasured = true
		if n := resp.Usage.CompletionTokens; n > 0 {
			res.DecodePerSecond = float64(n) / secs
		}
	}
	if tau, ok := resp.AcceptanceLength(); ok {
		res.AcceptanceLength, res.AcceptanceMeasured = tau, true
		res.DraftN, res.DraftAccepted = *resp.Timings.DraftN, *resp.Timings.DraftNAccepted
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
	// Through the profile: a backend that inlines its reasoning in the content would
	// otherwise have it counted as answer text and scored as one.
	reasoning, content := msg.ReasoningContent, msg.Content
	if prof != nil {
		reasoning, content = prof.ExtractReasoning(msg.ReasoningContent, msg.Content)
	}
	msg.Content = content
	res.ReasoningChars = len(reasoning)

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
