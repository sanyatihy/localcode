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

// gate counts what it permits, not what it is asked. A denied call spends nothing, so a
// session cannot be pushed past its budget by calls that never ran.
func gate(p Payload, spec Spec, dir string) Verdict {
	s := State{Peak: Peak(p.TranscriptPath)}
	// Counted before it is decided, not after. The harness runs a turn's calls
	// concurrently, so several of these are deciding at once; taking a number first is what
	// gives each of them a different one to decide against.
	if IsHandoff(p.ToolName, p.ToolInput, spec.Handoff) {
		s.Handoffs = Bump(dir, HandoffsFile(p.SessionID)) - 1
	} else {
		s.Calls = Bump(dir, CallsFile(p.SessionID)) - 1
		s.Batch = Bump(dir, BatchFile(p.SessionID, s.Peak)) - 1
	}
	return Gate(p, spec, s)
}

// stop counts its refusals rather than reading the harness's stop_hook_active, because the
// count is what the bound is expressed in and the counter is per session id in a directory
// that belongs to one session.
func stop(p Payload, spec Spec, dir string) Verdict {
	v := Stop(Read(spec.Handoff), spec.Handoff, Counter(dir, StopFile(p.SessionID)))
	if v.Deny {
		Bump(dir, StopFile(p.SessionID))
	}
	return v
}
