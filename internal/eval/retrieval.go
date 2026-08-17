package eval

import (
	"fmt"
	"math/rand"
	"strings"
)

// Retrieval tasks bury a sentinel in filler and ask for it back. They exist to catch
// damage that only shows at depth — KV cache quantisation above all, which is cheap
// on memory and is exactly the setting that could silently cost long-context recall.
// A tool-call task at 400 tokens would never see it.
type Retrieval struct {
	DepthTokens int     `json:"depth_tokens"` // approximate prompt size to build
	Position    float64 `json:"position"`     // 0.0 start, 0.5 middle, 1.0 end
	Sentinel    string  `json:"sentinel"`     // the string the answer must contain
	Question    string  `json:"question"`
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

// buildHaystack is seeded from the task's own depth so a given task produces the
// same prompt on every run. Comparing configs against different filler would be
// comparing noise.
func buildHaystack(r Retrieval) string {
	rng := rand.New(rand.NewSource(int64(r.DepthTokens)))
	words := r.DepthTokens // ~1 token per word for this vocabulary, close enough
	if words < 100 {
		words = 100
	}
	pos := int(float64(words) * clamp01(r.Position))

	var b strings.Builder
	b.WriteString("Repository index dump follows.\n\n")
	for i := 0; i < words; i++ {
		if i == pos {
			fmt.Fprintf(&b, "\nNOTE: the deployment key is %s\n\n", r.Sentinel)
		}
		b.WriteString(fillerVocab[rng.Intn(len(fillerVocab))])
		b.WriteByte(' ')
	}
	if pos >= words {
		fmt.Fprintf(&b, "\nNOTE: the deployment key is %s\n", r.Sentinel)
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

func checkRetrieval(r Retrieval, content string) (Outcome, string) {
	if strings.TrimSpace(content) == "" {
		return FailEmpty, "empty answer"
	}
	if strings.Contains(content, r.Sentinel) {
		return Pass, ""
	}
	return FailRetrieval, fmt.Sprintf("sentinel %q absent from %q", r.Sentinel, truncate(strings.TrimSpace(content), 120))
}
