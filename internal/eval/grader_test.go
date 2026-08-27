package eval

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The grader compiles and runs code the model wrote, in a repository that refuses to run
// an agent at all when the sandbox is missing. These hold the two halves of what it is
// allowed to reach.

// `GOPROXY=off` is the half that holds on every platform: an import the model invented
// fails against the module cache rather than being fetched. Without it the grader spends
// its budget on a network round trip for a package that does not exist.
func TestAnInventedImportFailsWithoutReachingForIt(t *testing.T) {
	p := Patch{
		Dir:      filepath.Join("..", "..", "tasks", "patch-off-by-one"),
		Source:   "broken.go.txt",
		TestFile: "verify_test.go.txt",
	}
	got, detail := runPatch(context.Background(), p, `package main

import "example.com/nothing/here/v9"

func Median(xs []float64) float64 { return nothing.Here(xs) }`)
	if got != FailCompile {
		t.Fatalf("an invented import must fail to build, got %q (%s)", got, detail)
	}
	// The distinction that matters: refused locally, not fetched and then refused.
	if strings.Contains(detail, "proxy.golang.org") || strings.Contains(detail, "lookup ") {
		t.Fatalf("the grader reached for the module: %s", detail)
	}
}

// The other half is macOS's. Wrapping is skipped rather than refused where sandbox-exec
// is absent — CI is Linux — so what is asserted is that each state produces the command
// it should, not that the boundary exists everywhere.
func TestTheGraderIsWrappedWhereItCanBeAndRunsPlainWhereItCannot(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		stub := filepath.Join(t.TempDir(), "sandbox-exec")
		if err := os.WriteFile(stub, []byte("#!/bin/sh\nshift 2\nexec \"$@\"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		was := sandboxExec
		sandboxExec = stub
		t.Cleanup(func() { sandboxExec = was })

		name, args, done := denyNetwork("go", []string{"test", "./..."})
		defer done()
		if name != stub {
			t.Fatalf("the grader must run under the sandbox: %s", name)
		}
		i := slices.Index(args, "-f")
		if i < 0 || i+1 >= len(args) {
			t.Fatalf("no profile is named: %v", args)
		}
		body, err := os.ReadFile(args[i+1])
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "(deny network*)") {
			t.Fatalf("the profile must deny the network: %s", body)
		}
		// The model is served on loopback, which is the same line harness/offline.sb draws.
		if !strings.Contains(string(body), `(remote ip "localhost:*")`) {
			t.Fatalf("the profile must leave loopback open: %s", body)
		}
		profile := args[i+1]
		done()
		if _, err := os.Stat(profile); !os.IsNotExist(err) {
			t.Fatalf("the profile outlived the run it was written for: %v", err)
		}
	})

	t.Run("absent", func(t *testing.T) {
		was := sandboxExec
		sandboxExec = filepath.Join(t.TempDir(), "no-sandbox-here")
		t.Cleanup(func() { sandboxExec = was })

		name, args, done := denyNetwork("go", []string{"test", "./..."})
		defer done()
		if name != "go" || !slices.Equal(args, []string{"test", "./..."}) {
			t.Fatalf("with no sandbox the command runs as it was: %s %v", name, args)
		}
	})
}

// Tier 2 hands the scratch module to the harness as its checkout, so anything the grader
// leaves there is something the model can read — and the fixture describes what is meant
// to be in it.
func TestTheGraderLeavesNothingInTheModuleTheHarnessIsGiven(t *testing.T) {
	stub := filepath.Join(t.TempDir(), "sandbox-exec")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nshift 2\nexec \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	was := sandboxExec
	sandboxExec = stub
	t.Cleanup(func() { sandboxExec = was })

	work := t.TempDir()
	kept := map[string]string{
		"go.mod":         "module scratch\n\ngo 1.26\n",
		"answer.go":      "package main\n\nfunc Sum(a, b int) int { return a + b }\n",
		"verify_test.go": "package main\n\nimport \"testing\"\n\nfunc TestSum(t *testing.T) {\n\tif Sum(1, 2) != 3 {\n\t\tt.Fatal(\"no\")\n\t}\n}\n",
	}
	for name, body := range kept {
		if err := os.WriteFile(filepath.Join(work, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := goTest(context.Background(), work); err != nil {
		t.Fatalf("the fixture module must build: %v", err)
	}
	entries, err := os.ReadDir(work)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if _, want := kept[e.Name()]; !want {
			t.Fatalf("the grader left %s where the harness will read it", e.Name())
		}
	}
}
