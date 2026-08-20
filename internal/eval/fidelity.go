package eval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Lossless is a claim about an algorithm, not about a build. A speculative decoder is
// supposed to commit exactly the tokens the target would have produced alone, so the
// check is direct: run the same greedy prompts with the mechanism on and off and compare
// what came back. A mismatch voids the run rather than scoring it — a faster decoder that
// changes the answer is not the thing being measured.
//
// Greedy on purpose. At a sampling temperature two identical configurations diverge on
// their own and the comparison says nothing; at temperature zero with top_k 1 the target
// has one answer, and so must the speculating build.
//
// The prompts are fixed here rather than read from tasks/: they are an instrument, they
// must not change when the suite does, and a hash is only comparable against the same
// questions.
func FidelityProbes() [][]Message {
	return [][]Message{
		{{Role: "user", Content: "Count from 1 to 20, separated by spaces. Reply with the numbers only."}},
		{{Role: "user", Content: "Write a Go function named Sum that adds two ints and returns the result. Code only."}},
		{{Role: "user", Content: "List the first eight prime numbers, comma separated. Numbers only."}},
	}
}

// FidelityResult is the token-for-token evidence for one config.
type FidelityResult struct {
	Hash    string // over every probe's reply, in order
	Replies []string
}

// Fidelity runs the probes greedily and hashes what came back. Any transport failure is
// returned rather than hashed: a check that silently hashes an error message would match
// itself across two builds and report fidelity where there was none.
func Fidelity(ctx context.Context, c *Client, maxTokens int) (FidelityResult, error) {
	zero, one := 0.0, 1
	greedy := Sampling{Temperature: &zero, TopK: &one}
	sum := sha256.New()
	var out FidelityResult
	for i, msgs := range FidelityProbes() {
		resp, err := c.Complete(ctx, chatRequest{
			Messages: msgs, MaxTokens: maxTokens, Sampling: greedy,
			ChatTemplateKwargs: map[string]any{"enable_thinking": false},
		})
		if err != nil {
			return FidelityResult{}, fmt.Errorf("fidelity probe %d: %w", i, err)
		}
		if len(resp.Error) > 0 && string(resp.Error) != "null" {
			return FidelityResult{}, fmt.Errorf("fidelity probe %d: server error: %s", i, truncate(string(resp.Error), 200))
		}
		if len(resp.Choices) == 0 {
			return FidelityResult{}, fmt.Errorf("fidelity probe %d: no choices returned", i)
		}
		reply := resp.Choices[0].Message.Content
		out.Replies = append(out.Replies, reply)
		_, _ = sum.Write([]byte(reply))
		_, _ = sum.Write([]byte{0})
	}
	out.Hash = hex.EncodeToString(sum.Sum(nil))
	return out, nil
}
