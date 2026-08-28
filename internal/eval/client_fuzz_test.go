package eval

import "testing"

// A missing counter is an error here rather than a zero, because a zero reads as a run that
// cost nothing. What must not happen is a sample reported as available while carrying a
// count no server could have emitted.
func FuzzParseMetrics(f *testing.F) {
	f.Add("llamacpp:prompt_tokens_total 12\nllamacpp:prompt_tokens_cached_total 3\n" +
		"llamacpp:tokens_predicted_total 4\n")
	f.Add("# a comment\n")
	f.Add("llamacpp:prompt_tokens_total -1\n")
	f.Fuzz(func(t *testing.T, body string) {
		m, err := parseMetrics(body)
		if err != nil {
			if m.Available {
				t.Fatalf("a failed read reported itself available: %+v", m)
			}
			return
		}
		if !m.Available {
			t.Fatalf("a successful read reported itself unavailable: %+v", m)
		}
		if m.PromptTokens < 0 || m.CachedTokens < 0 || m.PredictedTokens < 0 {
			t.Fatalf("negative counter %+v from: %q", m, body)
		}
	})
}
