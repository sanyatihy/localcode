package eval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Patch tasks hand the model broken code and check the answer by compiling it and
// running a test it never sees. That is the strongest deterministic signal available
// here: no string matching against a reference solution, so any correct fix passes
// and a plausible-looking wrong one does not.
// Fixture sources carry a .txt suffix so the go tool does not compile them as part
// of this module. They are deliberately broken — left as .go, `go test ./...` on this
// repo would build and fail them, and the gate that proves the scorer works would be
// permanently red for the wrong reason.
type Patch struct {
	Dir      string `json:"-"`         // fixture directory, filled in at load
	Source   string `json:"source"`    // file the model must rewrite, e.g. broken.go.txt
	TestFile string `json:"test_file"` // unseen test run against the answer, e.g. verify_test.go.txt
}

var fenceRE = regexp.MustCompile("(?s)```(?:go|golang)?\\s*\n(.*?)```")

// extractCode pulls Go source out of a reply. Models wrap code in fences most of the
// time and occasionally do not, so both are accepted — refusing unfenced code would
// score a formatting habit as a wrong answer.
func extractCode(reply string) string {
	if m := fenceRE.FindStringSubmatch(reply); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(reply)
}

// runPatch writes the model's code beside the unseen test and runs `go test`.
// Compilation failure and test failure are reported apart: one is the model
// producing invalid Go, the other is it producing Go that is wrong.
func runPatch(ctx context.Context, p Patch, code string) (Outcome, string) {
	if code == "" {
		return FailEmpty, "no code in reply"
	}
	work, err := os.MkdirTemp("", "localcode-patch-")
	if err != nil {
		return FailServer, fmt.Sprintf("tempdir: %v", err)
	}
	defer func() { _ = os.RemoveAll(work) }()

	testSrc, err := os.ReadFile(filepath.Join(p.Dir, p.TestFile))
	if err != nil {
		return FailServer, fmt.Sprintf("fixture test unreadable: %v", err)
	}
	const gomod = "module localcodepatch\n\ngo 1.26\n"
	// Written under fixed .go names: the fixture keeps a .txt suffix to stay out of
	// this module, but the scratch module needs real Go filenames to compile.
	writes := map[string][]byte{
		"go.mod":         []byte(gomod),
		"answer.go":      []byte(code),
		"verify_test.go": testSrc,
	}
	for name, data := range writes {
		if err := os.WriteFile(filepath.Join(work, name), data, 0o644); err != nil {
			return FailServer, fmt.Sprintf("write %s: %v", name, err)
		}
	}

	// Bounded: a model can emit code that compiles and then loops forever. That is a
	// failed answer, not a broken harness, so it is scored rather than fatal.
	runCtx, cancel := context.WithTimeout(ctx, patchTestBudget)
	defer cancel()
	out, err := goTest(runCtx, work)
	switch {
	case err == nil:
		return Pass, ""
	case runCtx.Err() != nil:
		return FailTest, fmt.Sprintf("test run exceeded %s (likely non-terminating)", patchTestBudget)
	case isBuildFailure(out):
		// Kept apart from a test failure: one is invalid Go, the other is Go that is wrong.
		return FailCompile, truncate(firstUseful(string(out)), 200)
	}
	return FailTest, truncate(firstUseful(string(out)), 200)
}

// patchTestBudget bounds one scored answer. Well above the seconds a fixture's own tests
// take, and short enough that a non-terminating answer costs a minute rather than a sweep.
const patchTestBudget = 90 * time.Second

// firstUseful skips go's noise lines so the recorded detail is the actual error.
func firstUseful(s string) string {
	for _, line := range strings.Split(s, "\n") {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "#") || l == "FAIL" || strings.HasPrefix(l, "ok ") {
			continue
		}
		return l
	}
	return strings.TrimSpace(s)
}
