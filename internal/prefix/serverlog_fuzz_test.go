package prefix

import (
	"strings"
	"testing"
)

// The cache figure is derived rather than printed, which makes it the one number a log this
// does not expect can turn into nonsense that still looks like a measurement. `go test` runs
// a target's seed corpus and everything under testdata/fuzz on an ordinary run, so what the
// fuzzer finds becomes a permanent test at no cost to the gate.
func FuzzRequests(f *testing.F) {
	f.Add(oneRequest)
	f.Add("task 8processing task\ntask 8eval timems /7\ntask 8stop processingn_tokens =0\n")
	f.Add("")
	f.Fuzz(func(t *testing.T, log string) {
		rows, impossible, err := Requests(strings.NewReader(log), "cfg", "sess")
		if err != nil {
			return
		}
		if impossible < 0 {
			t.Fatalf("negative drop count %d", impossible)
		}
		for _, r := range rows {
			// Every row that survives is one a request could have produced.
			if !r.possible() {
				t.Fatalf("impossible row survived: %+v from:\n%s", r, log)
			}
			if r.HitShare < 0 || r.HitShare > 1 {
				t.Fatalf("hit share %v outside [0,1]: %+v", r.HitShare, r)
			}
			// A prompt is what was read, so its two halves cannot exceed it.
			if r.IngestedTokens+r.CachedTokens != r.PromptTokens {
				t.Fatalf("ingested+cached != prompt: %+v", r)
			}
		}
	})
}
