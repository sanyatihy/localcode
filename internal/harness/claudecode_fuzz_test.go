package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The committed environment decides the window every session is budgeted against, so a line
// this reads wrongly serves a different configuration under the same label. It is strict on
// purpose: what must not happen is a variable coming back in a shape the child cannot use.
func FuzzParseEnvFile(f *testing.F) {
	f.Add("A=1\n# comment\nB=\"two\"\n")
	f.Add("=novalue\n")
	f.Add("CLAUDE_CODE_MAX_CONTEXT_TOKENS=45056\n")
	f.Fuzz(func(t *testing.T, body string) {
		path := filepath.Join(t.TempDir(), "env")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Skip()
		}
		vars, err := parseEnvFile(path)
		if err != nil {
			return
		}
		for _, kv := range vars {
			name, _, ok := strings.Cut(kv, "=")
			if !ok || name == "" {
				t.Fatalf("entry %q is not usable as an environment variable: %q", kv, body)
			}
		}
	})
}
