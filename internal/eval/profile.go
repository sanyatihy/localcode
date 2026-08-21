package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// A Profile is everything about driving one model that is not the same for every model:
// how thinking is switched, what sampling each mode wants, and where the reasoning comes
// back. All three are read — a field here that the scorer does not consult would be a
// config file claiming an abstraction the code does not have.
//
// The sampling pair is here rather than in a flag default because the pair is a property
// of the model: each mode has its own, and sweeping the toggle at one fixed temperature
// measures the pair instead of the toggle. That has already happened once, to 114 rows.
type Profile struct {
	Name      string              `json:"name"`
	Note      string              `json:"note,omitempty"`
	Thinking  ThinkingSpec        `json:"thinking"`
	Sampling  map[string]Sampling `json:"sampling"`
	Reasoning ReasoningSpec       `json:"reasoning"`
}

// ThinkingSpec says how this model is told to think, which is not portable: Qwen3.8 takes a
// chat-template keyword and the next model will take something else.
//
// The effort *level* is deliberately absent. It is passed to the server verbatim rather than
// checked against a list, because the next model's vocabulary differs and the server rejects
// what it does not know; a list here would be a gate this project has decided not to have.
type ThinkingSpec struct {
	Mechanism string `json:"mechanism"` // chat_template_kwarg
	Key       string `json:"key"`
}

// MechanismChatTemplateKwarg is the only mechanism implemented. A profile naming another is
// refused at load rather than ignored at request time.
const MechanismChatTemplateKwarg = "chat_template_kwarg"

// ReasoningSpec says where reasoning arrives. Some servers return it in a field of its
// own; others leave it inline in the content wrapped in a tag. Both are supported because
// both are already in use across the backends this project intends to score.
type ReasoningSpec struct {
	ResponseField string `json:"response_field,omitempty"`
	InlineTag     string `json:"inline_tag,omitempty"`
}

const (
	ModeThinking    = "thinking"
	ModeNonThinking = "nonthinking"
)

func LoadProfile(path string) (*Profile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("%s: profile needs a name", path)
	}
	for _, mode := range []string{ModeThinking, ModeNonThinking} {
		if _, ok := p.Sampling[mode]; !ok {
			return nil, fmt.Errorf("%s: profile has no %q sampling; a toggle with only one "+
				"side defined is how a sweep ends up measuring the pair", path, mode)
		}
	}
	if p.Thinking.Mechanism != MechanismChatTemplateKwarg || p.Thinking.Key == "" {
		return nil, fmt.Errorf("%s: thinking.mechanism must be %q with a key; %q is not implemented "+
			"and a profile that names it would leave the toggle unset while a row claimed otherwise",
			path, MechanismChatTemplateKwarg, p.Thinking.Mechanism)
	}
	return &p, nil
}

// ThinkingKwargs renders the toggle the way this model takes it. Errors rather than falling
// back: a run that could not set the toggle it was asked for would be recorded under a label
// saying it did, which is the mislabelling this type exists to prevent.
func (p *Profile) ThinkingKwargs(on bool) (map[string]any, error) {
	if p.Thinking.Mechanism != MechanismChatTemplateKwarg || p.Thinking.Key == "" {
		return nil, fmt.Errorf("profile %q cannot switch thinking", p.Name)
	}
	return map[string]any{p.Thinking.Key: on}, nil
}

// SamplingFor returns the model's own recommended sampling for a mode. Callers get an
// error rather than a zero value for an unknown mode: silently sampling at the server's
// default is the failure this type exists to prevent.
func (p *Profile) SamplingFor(mode string) (Sampling, error) {
	s, ok := p.Sampling[mode]
	if !ok {
		return Sampling{}, fmt.Errorf("profile %q has no sampling for mode %q", p.Name, mode)
	}
	return s, nil
}

// ExtractReasoning pulls the reasoning out of a reply, from whichever place this model
// puts it, and returns the content with it removed. A model that inlines its reasoning
// would otherwise have it counted as answer text — and scored as one.
func (p *Profile) ExtractReasoning(field, content string) (reasoning, rest string) {
	if p.Reasoning.ResponseField != "" && field != "" {
		return field, content
	}
	tag := p.Reasoning.InlineTag
	if tag == "" {
		return field, content
	}
	open, shut := "<"+tag+">", "</"+tag+">"
	i, j := strings.Index(content, open), strings.Index(content, shut)
	if i < 0 || j < i {
		return field, content
	}
	return content[i+len(open) : j], strings.TrimSpace(content[:i] + content[j+len(shut):])
}
