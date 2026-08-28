package chain

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SpecName is what a session's budget is written to, beside the handoff it will write.
const SpecName = "session.json"

// Spec is what one session runs under. A file beside the handoff rather than a handful of
// environment variables: the hooks are spawned by the harness rather than by the
// supervisor, and this is also the record of what a session's numbers were, which a
// results row would otherwise have to restate.
type Spec struct {
	Limits  Limits `json:"limits"`
	Handoff string `json:"handoff"` // absolute, and the only path the session may write out of turn
	Chain   string `json:"chain"`
	Session int    `json:"session"`
	// Harness is the agent this session ran in, recorded because what it left behind can
	// only be read by something that knows which one it was. Per session rather than per
	// chain: a resumed chain may be given a different one, and the rows either side of
	// that were written by different agents.
	Harness string `json:"harness,omitempty"`
	// OneShot is whether this session answers an instruction. Only such a session may be
	// refused permission to stop: an interactive one ends every time it hands the keyboard
	// back, and refusing there refuses the conversation itself. It lives here rather than
	// in each adapter because the supervisor is the only thing that knows, and an adapter
	// that has to remember is an adapter that can forget.
	OneShot bool `json:"one_shot,omitempty"`
}

func WriteSpec(dir string, s Spec) error {
	body, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, SpecName), body, 0o644)
}

func ReadSpec(dir string) (Spec, error) {
	body, err := os.ReadFile(filepath.Join(dir, SpecName))
	if err != nil {
		return Spec{}, err
	}
	var s Spec
	if err := json.Unmarshal(body, &s); err != nil {
		return Spec{}, fmt.Errorf("%s: %w", filepath.Join(dir, SpecName), err)
	}
	return s, nil
}

// LastSpec is what the newest session of a chain ran under, and whether the chain has one
// to compare against. Newest first with a fallback, because a session directory made for a
// run that died before it was budgeted would otherwise hide the session before it.
func LastSpec(dir string) (Spec, bool) {
	ns := Sessions(dir)
	for i := len(ns) - 1; i >= 0; i-- {
		if s, err := ReadSpec(filepath.Join(dir, fmt.Sprintf("%02d", ns[i]))); err == nil {
			return s, true
		}
	}
	return Spec{}, false
}

// Changed names every bound that differs between what the last session ran under and what
// the next one will, as `field was -> is`. Empty for a chain that did not move server,
// which is every chain that resumes on the config it started on.
//
// Every field, because each is a bound the session feels: the window it has, the ceiling
// the gate holds it to, what one result may add, and how many calls it may spend.
func (l Limits) Changed(from Limits) []string {
	var out []string
	for _, f := range []struct {
		name     string
		was, now int
	}{
		{"window", from.Window, l.Window},
		{"ceiling", from.Ceiling, l.Ceiling},
		{"result cap", from.ResultCap, l.ResultCap},
		{"calls", from.Calls, l.Calls},
		{"batch", from.Batch, l.Batch},
	} {
		if f.was != f.now {
			out = append(out, fmt.Sprintf("%s %d -> %d", f.name, f.was, f.now))
		}
	}
	return out
}

// HarnessIn is the agent a chain's sessions last ran in, and whether its record says so.
// A chain that ran before this was recorded says nothing, and a reader has to fall back to
// what it can — the alternative is refusing to account for chains that already exist.
func HarnessIn(chainDir string) (string, bool) {
	s, ok := LastSpec(chainDir)
	return s.Harness, ok && s.Harness != ""
}

// ErrNoSpec is a session nobody budgeted.
var ErrNoSpec = errors.New("no session spec")

// Hook runs one hook against the session directory it was given and returns what to do.
//
// A missing spec stands aside rather than refusing. The two directions fail differently:
// standing aside leaves a session behaving as it did before this feature, while refusing
// every call would make an unconfigured session useless without saying why.
func Hook(name string, payload io.Reader, dir string) (Verdict, error) {
	if dir == "" {
		return Verdict{}, ErrNoSpec
	}
	body, err := io.ReadAll(payload)
	if err != nil {
		return Verdict{}, fmt.Errorf("read the hook payload: %w", err)
	}
	var p Payload
	if err := json.Unmarshal(body, &p); err != nil {
		return Verdict{}, fmt.Errorf("decode the hook payload: %w", err)
	}
	spec, err := ReadSpec(dir)
	if err != nil {
		return Verdict{}, ErrNoSpec
	}

	switch name {
	case "gate":
		return gate(p, spec, dir), nil
	case "stop":
		return stop(p, spec, dir), nil
	default:
		return Verdict{}, fmt.Errorf("no such hook: %s", name)
	}
}

// gate takes a number before it decides, so that concurrent hooks decide against different
// ones. A call the turn's bound holds back is the only kind that costs nothing: past the
// session's own bounds every attempt is charged, which is what keeps a session that answers
// a refusal with another call from being refused forever.
func gate(p Payload, spec Spec, dir string) Verdict {
	s := State{Peak: peakOf(p)}
	// Every counter is bumped before it is decided on, never after. The harness runs a
	// turn's calls concurrently, so several of these are deciding at once, and taking a
	// number first is what gives each of them a different one to decide against.
	if IsHandoff(p.ToolName, p.ToolInput, spec.Handoff) {
		s.Handoffs = Bump(dir, HandoffsFile(p.SessionID)) - 1
		return Gate(p, spec, s)
	}
	// The turn's bound is asked first, and what it refuses is not charged to the session:
	// a call held back to be measured again is not a call the session chose to spend.
	spent := BumpBatch(dir, p.SessionID, s.Peak) - 1
	if v := Turn(spec.Limits, spent); v.Deny {
		return v
	}
	s.Calls = Bump(dir, CallsFile(p.SessionID)) - 1
	v := Gate(p, spec, s)
	if v.Deny || p.ToolName != "Read" {
		return v
	}
	// `Bash` takes its cap from the harness and `Read` has none, so the gate is where it
	// gets one. Narrowed rather than refused: the session reads what it asked for, up to
	// what the reserve holds for a call.
	if input, clamped := ClampRead(p.ToolInput, spec.Limits.ResultCap); clamped {
		v.Input = input
	}
	return v
}

// peakOf is the context the gate decides on: what the payload carries, or what the
// transcript says. One or the other, because two readers of one number are two answers to
// one question — and the harness that reports its own is the harness whose transcript this
// package cannot read.
func peakOf(p Payload) int {
	if p.PeakTokens > 0 {
		return p.PeakTokens
	}
	return Peak(p.TranscriptPath)
}

// stop counts its refusals rather than reading the harness's stop_hook_active, because the
// count is what the bound is expressed in and the counter is per session id in a directory
// that belongs to one session.
//
// A session nobody gave an instruction to is never refused. It ends every time it hands
// the keyboard back, so a refusal there is a refusal of the conversation: measured, an
// interactive Pi session was told twice that it had handed nothing on, because its
// extension asks on every `agent_end` and only the supervisor knows which kind of session
// this is.
func stop(p Payload, spec Spec, dir string) Verdict {
	if !spec.OneShot {
		return Verdict{}
	}
	v := Stop(Read(spec.Handoff), spec.Handoff, Counter(dir, StopFile(p.SessionID)))
	if v.Deny {
		Bump(dir, StopFile(p.SessionID))
	}
	return v
}
