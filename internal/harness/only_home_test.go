package harness

import (
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The property that makes a third harness additive: this package is the only one that
// knows which agent a chain is running. Everything above it works through the interface,
// so adding one touches the new adapter and the factory beside it — and nothing else.
//
// Checked rather than asserted, because it is the kind of property a single convenient
// line undoes: a transcript path in the driver, a flag string in a launcher, an event
// shape in a renderer. Each of those has been written here before.
//
// The scorer's own tools are deliberately outside this: `cmd/tier2` and `cmd/report`
// exist to compare harnesses by name, so naming them is what they are for.
var chainPackages = []string{
	filepath.Join("cmd", "localcode"),
	filepath.Join("internal", "chain"),
	filepath.Join("internal", "handoff"),
}

// Words that belong to one harness and to nothing else: what each binary is called, the
// API it speaks, the shape of the events it prints, the files it keeps.
var harnessWords = []string{
	"claude", "anthropic", // the incumbent, its API, its variables and its config directory
	"stream-json", "tool_use", "content_block", // its event stream
	"opencode", "hermes", // the two with scorer adapters and no chain one
	"local-provider", "pi_offline", "pi_coding_agent", // Pi's extension and its variables
}

func TestNoPackageAboveTheAdaptersNamesAHarness(t *testing.T) {
	for _, pkg := range chainPackages {
		dir := filepath.Join("..", "..", pkg)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			for _, word := range namesIn(t, filepath.Join(dir, name)) {
				t.Errorf("%s names the harness %q: what differs between agents belongs behind "+
					"harness.Agent, or a third one cannot be added without touching this file",
					filepath.Join(pkg, name), word)
			}
		}
	}
}

// namesIn is the file's code with its comments dropped. Comments are where a decision
// says which harness it was taken against — `internal/chain` explains what a Claude Code
// payload carries and why Pi's differs — and losing that would cost more than the check
// buys. Printing an AST parsed without ParseComments is what drops them.
func namesIn(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var code strings.Builder
	if err := printer.Fprint(&code, fset, file); err != nil {
		t.Fatalf("print %s: %v", path, err)
	}
	lower := strings.ToLower(code.String())
	var found []string
	for _, word := range harnessWords {
		if strings.Contains(lower, word) {
			found = append(found, word)
		}
	}
	return found
}
