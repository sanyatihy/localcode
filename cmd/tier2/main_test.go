package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sanyatihy/localcode/internal/eval"
)

func runTier2(t *testing.T, args ...string) error {
	t.Helper()
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = null.Close() })
	return run(args, null, null)
}

func repo(parts ...string) string {
	return filepath.Join(append([]string{"..", ".."}, parts...)...)
}

func TestRefusesWhatItCannotRun(t *testing.T) {
	fixture := repo("tasks", "patch-nil-check")
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"neither -fixture nor -fixtures", []string{"-drivers", "pi"}},
		{"both -fixture and -fixtures", []string{
			"-fixture", fixture, "-fixtures", repo("tasks"), "-drivers", "pi"}},
		{"no driver named", []string{"-fixture", fixture, "-drivers", ""}},
		{"a driver that does not exist", []string{"-fixture", fixture, "-drivers", "cursor"}},
		{"a desk profile the machine does not declare", []string{
			"-fixture", fixture, "-drivers", "pi", "-profile", "idle"}},
		{"a machine file that is not there", []string{
			"-fixture", fixture, "-drivers", "pi", "-machine", repo("config", "nope.json")}},
	} {
		if err := runTier2(t, tc.args...); err == nil {
			t.Errorf("%s: expected a refusal", tc.name)
		}
	}
}

// The profile is read from the machine, so an unknown one names what the machine does
// declare rather than falling back to a default — a row that cannot say which profile it was
// taken under cannot be compared with one that can. Pointed at the committed machine, so this
// asserts a property of the file a real run reads.
func TestUnknownProfileNamesTheOnesThatExist(t *testing.T) {
	err := runTier2(t, "-fixture", repo("tasks", "patch-nil-check"), "-drivers", "pi",
		"-machine", repo("config", "machine.json"), "-profile", "idle")
	if err == nil {
		t.Fatal("expected a refusal")
	}
	for _, want := range []string{"attended", "unattended"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q, so the reader cannot act on it: %v", want, err)
		}
	}
}

// Every path a driver needs is made absolute before any adapter runs, because adapters run
// with the working directory set to a scratch checkout — left relative, pi looks for its
// extension inside /tmp and fails with a message about the extension rather than the path.
func TestValidateMakesEveryConfiguredPathAbsolute(t *testing.T) {
	c := config{
		fixture: "tasks/patch-nil-check", drivers: []string{"pi"},
		piExtension: "harness/pi/local-provider.js",
		ocConfig:    "harness/opencode/opencode.json",
		ccEnv:       "harness/claude-code/claude-code.env",
		hermesCfg:   "harness/hermes/config.yaml.reference",
		sandbox:     "harness/offline.sb",
	}
	if err := c.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	for name, got := range map[string]string{
		"fixture": c.fixture, "pi-extension": c.piExtension, "opencode-config": c.ocConfig,
		"claude-code-env": c.ccEnv, "hermes-config": c.hermesCfg, "sandbox-profile": c.sandbox,
	} {
		if !filepath.IsAbs(got) {
			t.Errorf("%s stayed relative (%q); it would resolve against the scratch checkout", name, got)
		}
	}
	// An empty path is not a path and must not become the working directory.
	empty := config{fixture: "x", drivers: []string{"pi"}}
	if err := empty.validate(); err != nil {
		t.Fatalf("validate with unset optional paths: %v", err)
	}
	if empty.sandbox != "" {
		t.Errorf("an unset sandbox became %q, which would run every harness under it", empty.sandbox)
	}
}

func TestBuildDriversRefusesAnUnknownName(t *testing.T) {
	if _, err := buildDrivers(config{drivers: []string{"pi", "cursor"}}); err == nil {
		t.Error("an unknown driver must be refused, not silently dropped from the comparison")
	}
	ds, err := buildDrivers(config{drivers: []string{"pi", "opencode", "hermes", "claude-code"}})
	if err != nil {
		t.Fatalf("buildDrivers: %v", err)
	}
	if len(ds) != 4 {
		t.Fatalf("built %d drivers, want 4", len(ds))
	}
	// Only Hermes declares a floor; a floor appearing on another changes what the
	// comparison can say and must be a deliberate edit.
	floored := 0
	for _, d := range ds {
		if _, ok := d.(eval.ContextFloorer); ok {
			floored++
		}
	}
	if floored != 1 {
		t.Errorf("%d drivers declare a context floor, want 1 (hermes)", floored)
	}
}

func TestSplitNonEmpty(t *testing.T) {
	got := splitNonEmpty(" pi , ,opencode,")
	if len(got) != 2 || got[0] != "pi" || got[1] != "opencode" {
		t.Errorf("splitNonEmpty = %q, want [pi opencode]", got)
	}
	if len(splitNonEmpty(" , ")) != 0 {
		t.Error("a list of separators names no drivers")
	}
}
