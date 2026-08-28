package transcript

import (
	"os"
	"path/filepath"
	"testing"
)

// The peak this returns is what the gate holds a session to, so a peak that comes back
// negative is a budget that never fires.
func FuzzReadSession(f *testing.F) {
	f.Add(`{"type":"assistant","message":{"usage":{"input_tokens":10,"output_tokens":2}}}`)
	f.Add(`{"type":"assistant","message":{"usage":{"input_tokens":-1}}}`)
	f.Add("not json at all\n")
	f.Fuzz(func(t *testing.T, body string) {
		path := filepath.Join(t.TempDir(), "t.jsonl")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Skip()
		}
		s, err := ReadSession(path)
		if err != nil {
			return
		}
		if s.Peak < 0 || s.Turns < 0 {
			t.Fatalf("peak %d over %d turns from: %q", s.Peak, s.Turns, body)
		}
		calls, err := Requests(path)
		if err != nil {
			return
		}
		// A call cannot have read or written a negative number of tokens, and no call can
		// have cost more than the session's own peak.
		for _, c := range calls {
			if c.Ingest < 0 || c.Cached < 0 || c.Output < 0 {
				t.Fatalf("negative counts %+v from: %q", c, body)
			}
			if c.Ingest+c.Cached+c.Output > s.Peak {
				t.Fatalf("a call cost %d past the session peak %d: %q",
					c.Ingest+c.Cached+c.Output, s.Peak, body)
			}
		}
	})
}
