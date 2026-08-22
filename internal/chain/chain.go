// Package chain keeps one session away from the served wall and carries what it did to
// the next one.
//
// Nothing here asks the model for anything. A session told in prose to spend three
// commands reached compaction anyway, and one warned at 45% of its window acknowledged
// the warning and carried on. What a session may spend is therefore decided by refusing
// tool calls, which is not a request.
package chain

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Limits is what one session may spend. Every number is derived from the window the
// harness was declared, because the window is what is being protected: a cap carried over
// from another envelope either wastes the context or fails to bound it. Pi's 50 KB
// tool-result cap is around 12,500 tokens, which is the whole of a 12,288-token window.
type Limits struct {
	Window    int `json:"window"`     // the prompt budget: the declared context minus the output reservation
	Ceiling   int `json:"ceiling"`    // the context past which the gate permits only the handoff
	ResultCap int `json:"result_cap"` // bytes one tool result may add to the context
	Batch     int `json:"batch"`      // how many calls one turn may spend before the context is read again
	Calls     int `json:"calls"`      // the bound on tool calls, which binds when the context cannot be read
}

const (
	// bytesPerToken turns the token reserve into the byte cap the harness takes. Prose and
	// source both run near four bytes a token, and the cap is a bound: being a fifth out
	// costs headroom rather than correctness.
	bytesPerToken = 4

	// resultShare is the reciprocal of the fraction of the window that lands after the
	// gate's last decision. The gate reads the context as it stood before the results it
	// is permitting arrived, so the reserve has to cover all of them. A quarter, because an
	// eighth was measured too small: one turn's four permitted calls carried the context
	// 2,119 tokens past the reading they were decided on, which is 530 a call against the
	// 352 an eighth reserved. Only one of the tools has a cap of its own, so the reserve is
	// what covers a `Read` of something long.
	resultShare = 4

	// batchLimit is how many calls one assistant turn may spend. The harness issues a
	// turn's calls together and the transcript does not change while they run, so without
	// this the gate decides once and any number of results land on that one decision:
	// measured, a five-call turn carried the context 1,960 tokens past a ceiling it had
	// been under. It is what turns the reserve into a bound rather than a hope.
	batchLimit = 4

	// preambleFloor is the smallest peak measured across six enforced sessions: what a
	// session costs before it has done anything. A ceiling below it admits no work at all,
	// and a session started under one dies on `Prompt is too long` having achieved nothing.
	preambleFloor = 4512

	// handoffFloor is the size a handoff must clear to be one. Below it the file is a
	// heading and a blank line.
	handoffFloor = 120

	// handoffGrace is how many writes the handoff itself may take once the budget is
	// spent. Bounded for the reason the Stop hook is bounded: a session that cannot write
	// a handoff has to end rather than spin.
	handoffGrace = 3

	// outputFloor is what the harness keeps for a reply whatever it was told to keep.
	// Bisected against a declared 12,288 by padding a prompt to an exact token count and
	// reading whether it was refused before it was sent: with 1,024 declared the boundary
	// falls between 3,700 and 3,900 tokens of padding, and with 6,000 declared between
	// 1,500 and 2,500 — 1,800 lower, against the 1,904 that a floor of this size predicts.
	// So the reservation is the larger of the declaration and this, and a budget taken from
	// the declaration alone is 3,072 tokens too generous at the smaller one. Sessions run
	// on that budget did their work and then died on `Prompt is too long`.
	outputFloor = 4096

	// stopTries is how often the Stop hook may refuse before it relents and lets the
	// mechanical extractor be the floor. Measured: the model tried to stop twice without a
	// handoff and complied on the third.
	stopTries = 2
)

// NewLimits derives a session's budget from what the harness was declared, or refuses.
//
// It refuses rather than clamping because a window too small to work in is a
// configuration mistake with a one-line fix, and the session that discovers it instead
// spends a cold ingest to say `Prompt is too long`: declaring 8,192 against a 4,096
// output reservation leaves 4,096, which is less than the preamble.
func NewLimits(maxContext, maxOutput, ceilingPct, calls int) (Limits, error) {
	if maxContext <= 0 || maxOutput <= 0 {
		return Limits{}, fmt.Errorf("a declared window and an output reservation are both "+
			"needed to size a session: got %d and %d", maxContext, maxOutput)
	}
	if ceilingPct < 1 || ceilingPct > 100 {
		return Limits{}, fmt.Errorf("a ceiling of %d%% of the window is not one", ceilingPct)
	}
	if calls < 1 {
		return Limits{}, fmt.Errorf("a budget of %d tool calls runs nothing", calls)
	}

	// The prompt budget, which is the number a session actually has: the reply shares the
	// served context with the conversation, so the reservation is not available to it — and
	// the reservation is the larger of what was declared and what the harness keeps anyway.
	window := maxContext - max(maxOutput, outputFloor)
	perCall := window / (resultShare * batchLimit)
	reserve := perCall * batchLimit

	// What the gate can permit and still be sure the session survives to write its
	// handoff. Three things land after the reading it decides on: the results of the calls
	// it is permitting — a whole turn of them, which is what the batch bound makes finite —
	// the turn that asked for them, and the turn that answers the denial by writing the
	// handoff.
	headroom := window - reserve - 2*maxOutput

	// As high as the arithmetic allows, and no higher. A fraction below the headroom buys
	// no safety the reserve does not already buy, and it costs handoffs: one is about 4,265
	// tokens, so a smaller session is a worse session unless something makes it safer.
	// Measured at half the window, a session could not both read a file and edit it — which
	// `Edit` requires of it — so the chain wrote handoffs and never changed a line.
	ceiling := window * ceilingPct / 100
	if ceiling > headroom {
		ceiling = headroom
	}
	if ceiling < preambleFloor {
		return Limits{}, fmt.Errorf("a %d-token window with a %d-token reservation leaves a "+
			"ceiling of %d, under the %d a session costs before it does anything: serve more "+
			"context, or reserve less output",
			maxContext, maxOutput, ceiling, preambleFloor)
	}
	return Limits{Window: window, Ceiling: ceiling, ResultCap: perCall * bytesPerToken,
		Batch: batchLimit, Calls: calls}, nil
}

// Payload is the part of a hook's stdin that is read here.
type Payload struct {
	SessionID      string         `json:"session_id"`
	TranscriptPath string         `json:"transcript_path"`
	ToolName       string         `json:"tool_name"`
	ToolInput      map[string]any `json:"tool_input"`
}

// State is what the gate knows about the session at the moment of a call. Peak is -1 when
// there was no transcript to read, which is the case for the first call of a session.
type State struct {
	Calls    int
	Handoffs int
	Peak     int
}

// Verdict is what a hook does: refuse, with a reason the model is given verbatim, or stand
// aside.
type Verdict struct {
	Deny   bool
	Reason string
	// Input is a permitted call held to what the reserve allows, and nil when the call goes
	// through as it was made. Narrowing beats refusing where it is possible: the session
	// gets what it asked for, bounded, rather than an error to work around.
	Input map[string]any
}

// Gate decides whether a session may spend another tool call.
//
// The reason states the session's state and stops there. A hook that told the model what
// to do was refused as injection — correctly, because it arrived through a tool result.
// What to do about a spent budget belongs in the appended system prompt, which is trusted.
func Gate(p Payload, spec Spec, s State) Verdict {
	l := spec.Limits
	if IsHandoff(p.ToolName, p.ToolInput, spec.Handoff) {
		if s.Handoffs >= handoffGrace {
			return Verdict{Deny: true, Reason: fmt.Sprintf(
				"localcode: the handoff has been written %d times and this session is over.",
				s.Handoffs)}
		}
		return Verdict{}
	}
	switch {
	case s.Peak >= l.Ceiling:
		return Verdict{Deny: true, Reason: fmt.Sprintf(
			"localcode: this session's context reached %d tokens of a %d ceiling. No call "+
				"other than writing the handoff will be permitted.", s.Peak, l.Ceiling)}
	case s.Calls >= l.Calls:
		return Verdict{Deny: true, Reason: fmt.Sprintf(
			"localcode: this session has spent %d of %d tool calls. No call other than "+
				"writing the handoff will be permitted.", s.Calls, l.Calls)}
	}
	return Verdict{}
}

// Turn decides whether a turn may spend another call, and is asked before the session's
// budget is. A throttle, not an end: what it refuses costs the session nothing, because a
// turn held to its bound has not decided to do less work — only to be measured again first.
func Turn(l Limits, spent int) Verdict {
	if spent < l.Batch {
		return Verdict{}
	}
	return Verdict{Deny: true, Reason: fmt.Sprintf(
		"localcode: this turn has spent its %d tool calls. The bound is per turn, and the "+
			"context is read again before the next one.", l.Batch)}
}

// Stop decides whether a session may end, and refuses until it has left something a fresh
// session could act on. It relents after two refusals so an autonomous run cannot be
// wedged by the hook that exists to protect it; the mechanical extractor is the floor
// under that.
func Stop(handoff []byte, path string, tries int) Verdict {
	if usable(handoff) || tries >= stopTries {
		return Verdict{}
	}
	return Verdict{Deny: true, Reason: fmt.Sprintf(
		"localcode: %s is missing, shorter than %d bytes, or carries no `**Next:**` line, "+
			"so this session has handed nothing on.", path, handoffFloor)}
}

// usable reports whether a handoff is one. Size and a `**Next:**` line, because those are
// what the failure looked like: a chain of eight sessions wrote eight handoffs of which
// seven carried `Prompt is too long` as their next step.
func usable(handoff []byte) bool {
	return len(handoff) >= handoffFloor && bytes.Contains(handoff, []byte("**Next:**"))
}

// IsHandoff reports whether a call is the session writing its way out. It is never part of
// the work, so it is never denied for the reasons work is.
//
// The one path the session was given, not any file named like it: a spent budget opens
// exactly one door, and "somewhere called HANDOFF.md" is a wider door than the supervisor
// will ever read from.
func IsHandoff(tool string, input map[string]any, handoff string) bool {
	if tool != "Write" && tool != "Edit" {
		return false
	}
	path, _ := input["file_path"].(string)
	if path == "" || handoff == "" {
		return false
	}
	return filepath.Clean(path) == filepath.Clean(handoff)
}

// HandoffName is the file, everywhere. One name so the gate, the supervisor and the two
// hooks 0016 already ships are talking about the same file.
const HandoffName = "HANDOFF.md"

// Next is the one line a supervisor reads back out of a handoff: what the session that
// wrote it meant to do next. Empty when the handoff carries none.
func Next(handoff []byte) string {
	for _, line := range strings.Split(string(handoff), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "**Next:**"); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// Done reports whether a handoff says the instruction is finished. The word is the
// chain's only completion signal, which is why the system prompt names it and the
// supervisor does not guess from anything else.
//
// The first clause, not the whole line. A session that has finished says so and then
// explains: measured on real work, `**Next:** none — 0003 is done. Remaining: human
// merges…` read as unfinished, which cost the chain an entire extra session and made it
// report its bound instead of its success. Demanding one bare word demands the model be
// terse about the one thing it most wants to justify.
//
// A qualifier after the word is allowed only from a short list, because "none of the
// tests pass" is the opposite of done and starts the same way.
func Done(handoff []byte) bool {
	head := strings.ToLower(strings.TrimSpace(clause(Next(handoff))))
	words := strings.Fields(head)
	if len(words) == 0 {
		return false
	}
	switch strings.Trim(words[0], " .,;:`*-—–") {
	case "none", "nothing", "done", "no":
	default:
		return false
	}
	if len(words) == 1 {
		return true
	}
	switch words[1] {
	case "for", "left", "further", "more", "remaining", "else", "to", "needed",
		"required", "pending", "outstanding":
		return true
	}
	return false
}

// clause is the part of a line before it starts explaining itself, and "" for a line that
// is only punctuation or nothing at all.
func clause(line string) string {
	parts := strings.FieldsFunc(line, func(r rune) bool {
		return strings.ContainsRune(".;:,—–", r)
	})
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

// ClampRead holds a `Read` to what the gate reserved for one call, and reports whether it
// changed anything.
//
// `Bash` takes its cap from the harness and `Read` has none of its own: its default bound
// is two thousand lines, which is a bound on lines rather than on the window. The clamp is
// computed from the file rather than from a tokens-per-line guess — how many lines of this
// file fit in the reserve is a question the file answers.
//
// The count is of the file's own bytes, and the harness numbers the lines it returns, so a
// clamped result lands a few percent above the cap: measured, 3,229 bytes against 3,072.
// The reserve carries that, and counting the prefixes here would be guessing at the
// harness's formatting.
func ClampRead(input map[string]any, capBytes int) (map[string]any, bool) {
	path, _ := input["file_path"].(string)
	if path == "" || capBytes <= 0 {
		return nil, false
	}
	offset := intOf(input["offset"], 1)
	lines, err := linesWithin(path, offset, capBytes)
	if err != nil {
		return nil, false // unreadable here is the tool's problem to report, not the gate's
	}
	// The model's own limit binds when it is the smaller: a session that asked for ten
	// lines wanted ten, and widening it would spend the window on its behalf.
	if want := intOf(input["limit"], 0); want > 0 && want <= lines {
		return nil, false
	}
	if lines <= 0 {
		lines = 1 // a line too long for the reserve is still the smallest read there is
	}
	out := make(map[string]any, len(input)+1)
	for k, v := range input {
		out[k] = v
	}
	out["limit"] = lines
	return out, true
}

// linesWithin counts how many lines from offset fit in a byte budget, and stops counting
// once they do not: the answer is a small number and the file may not be.
func linesWithin(path string, offset, capBytes int) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	at, spent, lines := 1, 0, 0
	for scan.Scan() {
		if at < offset {
			at++
			continue
		}
		spent += len(scan.Bytes()) + 1
		if spent > capBytes {
			break
		}
		lines++
		at++
	}
	if err := scan.Err(); err != nil {
		return 0, err
	}
	// The whole of what was asked for fits, so there is nothing to clamp.
	if spent <= capBytes {
		return 0, errNothingToClamp
	}
	return lines, nil
}

var errNothingToClamp = errors.New("the read fits the reserve")

func intOf(v any, fallback int) int {
	switch n := v.(type) {
	case float64: // every number out of encoding/json
		return int(n)
	case int:
		return n
	}
	return fallback
}
