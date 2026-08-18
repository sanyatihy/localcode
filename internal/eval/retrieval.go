package eval

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
)

// Retrieval tasks bury a sentinel in filler and ask for it back. They exist to catch
// damage that only shows at depth — KV cache quantisation above all, which is cheap
// on memory and is exactly the setting that could silently cost long-context recall.
// A tool-call task at 400 tokens would never see it.
//
// With Distractors set, the task stops measuring recall and starts measuring
// discrimination: several near-identical sentinels are present and the question names
// the wanted one by a property. Recalling *a* key is then no longer the same as
// answering, which is what 0013 needs — a plain recall task cannot express "worse"
// because a shallow read of it is also the correct one.
type Retrieval struct {
	DepthTokens int     `json:"depth_tokens"` // approximate prompt size to build
	Position    float64 `json:"position"`     // 0.0 start, 0.5 middle, 1.0 end
	Sentinel    string  `json:"sentinel"`     // the string the answer must contain
	Question    string  `json:"question"`

	// Label qualifies the sentinel where distractors need telling apart, e.g.
	// "staging". Empty keeps the original unqualified phrasing, so the fixtures
	// written before distractors existed generate byte-identical prompts.
	Label string `json:"label,omitempty"`

	// Distractors are wrong answers planted in the same haystack. An answer naming
	// one of them fails, so listing every key found is a failure and not a hedge.
	Distractors []Decoy `json:"distractors,omitempty"`
}

// Decoy is a planted wrong answer: same shape as the real sentinel, different label.
type Decoy struct {
	Label    string  `json:"label"`
	Sentinel string  `json:"sentinel"`
	Position float64 `json:"position"`
}

// filler vocabulary is code-shaped rather than prose: this measures recall in the
// kind of context an agent actually carries, and repetitive identifier-like tokens
// are closer to a repo dump than English is.
var fillerVocab = []string{
	"session", "token", "refresh", "handler", "request", "context", "buffer",
	"index", "commit", "parser", "value", "result", "config", "client", "server",
	"stream", "record", "module", "target", "branch", "queue", "worker", "socket",
	"logger", "schema", "cursor", "packet", "thread", "memory", "kernel",
}

// note renders one planted line. The unlabelled form is preserved exactly as it was
// before distractors existed: changing it would silently invalidate every retrieval
// number already recorded.
func note(label, sentinel string) string {
	if label == "" {
		return fmt.Sprintf("\nNOTE: the deployment key is %s\n\n", sentinel)
	}
	return fmt.Sprintf("\nNOTE: the %s deployment key is %s\n\n", label, sentinel)
}

type planted struct {
	at   int
	text string
}

// buildHaystack is seeded from the task's own depth so a given task produces the
// same prompt on every run. Comparing configs against different filler would be
// comparing noise. Insertions do not draw from the generator, so adding distractors
// to a task does not disturb the filler of any other.
func buildHaystack(r Retrieval) string {
	rng := rand.New(rand.NewSource(int64(r.DepthTokens)))
	words := r.DepthTokens // ~1 token per word for this vocabulary, close enough
	if words < 100 {
		words = 100
	}

	all := []planted{{at: int(float64(words) * clamp01(r.Position)), text: note(r.Label, r.Sentinel)}}
	for _, d := range r.Distractors {
		all = append(all, planted{at: int(float64(words) * clamp01(d.Position)), text: note(d.Label, d.Sentinel)})
	}
	// Stable so that two decoys sharing a position keep their fixture order, and the
	// prompt stays reproducible rather than depending on map or sort luck.
	sort.SliceStable(all, func(i, j int) bool { return all[i].at < all[j].at })

	var b strings.Builder
	b.WriteString("Repository index dump follows.\n\n")
	next := 0
	for i := 0; i < words; i++ {
		for next < len(all) && all[next].at <= i {
			b.WriteString(all[next].text)
			next++
		}
		b.WriteString(fillerVocab[rng.Intn(len(fillerVocab))])
		b.WriteByte(' ')
	}
	for ; next < len(all); next++ {
		b.WriteString(all[next].text)
	}
	return b.String()
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// checkRetrieval scores an answer. A distractor is checked for first and on purpose:
// an answer that recites every key in the dump contains the right one too, and
// scoring that as a pass would let a model dodge the discrimination the task exists
// to measure.
func checkRetrieval(r Retrieval, content string) (Outcome, string) {
	if strings.TrimSpace(content) == "" {
		return FailEmpty, "empty answer"
	}
	for _, d := range r.Distractors {
		if strings.Contains(content, d.Sentinel) {
			return FailRetrieval, fmt.Sprintf("answered with the %s key %q, wanted the %s key %q",
				d.Label, d.Sentinel, r.Label, r.Sentinel)
		}
	}
	if strings.Contains(content, r.Sentinel) {
		return Pass, ""
	}
	return FailRetrieval, fmt.Sprintf("sentinel %q absent from %q", r.Sentinel, truncate(strings.TrimSpace(content), 120))
}
