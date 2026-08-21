package eval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func qwenProfile(t *testing.T) *Profile {
	t.Helper()
	p, err := LoadProfile(filepath.Join("..", "..", "config", "profiles", "qwen3.8.json"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// The proof that moving sampling into a profile changed nothing: the shipped result files
// record the sampling each run actually sent, so they are the baseline. If the profile
// disagrees with them, the profile is not reproducing the matrix it claims to.
func TestProfileReproducesTheShippedMatrix(t *testing.T) {
	p := qwenProfile(t)
	baselines := map[string]string{ // config label -> mode it was run in
		"0005-off":    ModeNonThinking,
		"0005-on-low": ModeThinking,
	}
	seen := map[string]bool{}
	f, err := os.Open(filepath.Join("..", "..", "docs", "data", "2026-08-18-m2max-32gb-0005-toggle.jsonl"))
	if err != nil {
		t.Skipf("baseline not present: %v", err)
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var row struct {
			Config   string   `json:"config"`
			Sampling Sampling `json:"sampling"`
		}
		if err := json.Unmarshal(sc.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		mode, ok := baselines[row.Config]
		if !ok {
			continue
		}
		seen[row.Config] = true
		want, err := p.SamplingFor(mode)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := json.Marshal(row.Sampling)
		exp, _ := json.Marshal(want)
		if string(got) != string(exp) {
			t.Fatalf("%s: recorded sampling %s, profile would send %s", row.Config, got, exp)
		}
	}
	for cfg := range baselines {
		if !seen[cfg] {
			t.Errorf("baseline file contained no rows for %s; the comparison proved nothing", cfg)
		}
	}
}

// A profile missing one side of the toggle must not load. Half a pair is how a sweep ends
// up sampling one mode at the server's default and calling it a comparison.
func TestProfileRefusesAHalfDefinedToggle(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "half.json")
	body := `{"name":"half","sampling":{"thinking":{"temperature":1.0}}}`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProfile(p); err == nil {
		t.Error("a profile defining only one mode loaded; it should not")
	}
}

// Reasoning arrives in a field on this backend and inline on others. Both are extracted,
// and inline reasoning is removed from the content rather than left to be scored as answer.
func TestProfileExtractsReasoningFromEitherPlace(t *testing.T) {
	p := qwenProfile(t)
	if r, rest := p.ExtractReasoning("thought hard", "the answer"); r != "thought hard" || rest != "the answer" {
		t.Errorf("response-field path: got (%q, %q)", r, rest)
	}
	// Same profile, a backend that inlines instead: no separate field arrives.
	r, rest := p.ExtractReasoning("", "<think>weighing it up</think>\nthe answer")
	if r != "weighing it up" {
		t.Errorf("inline reasoning not extracted: %q", r)
	}
	if rest != "the answer" {
		t.Errorf("inline reasoning left in the content, where it would be scored as the answer: %q", rest)
	}
	if r, rest := p.ExtractReasoning("", "no tags here"); r != "" || rest != "no tags here" {
		t.Errorf("untagged content disturbed: (%q, %q)", r, rest)
	}
}

// Swap must be read off the "used" label. Three numbers of the same shape sit on that
// line, and picking the wrong one records a plausible figure that is not the one the run
// depended on.
func TestSwapIsReadFromTheUsedField(t *testing.T) {
	s := sampleMemory()
	if !s.OK {
		t.Skip("platform did not answer")
	}
	if s.SwapUsedMB < 0 {
		t.Errorf("negative swap: %v", s.SwapUsedMB)
	}
	// The total is the first figure on the line and is always >= used; if the parser
	// grabbed it, this would be indistinguishable from a machine with all swap in use.
	out, err := exec.Command("sysctl", "-n", "vm.swapusage").Output()
	if err != nil {
		t.Skip(err)
	}
	fields := strings.Fields(string(out))
	var total float64
	for i, f := range fields {
		if f == "total" && i+2 < len(fields) {
			total, _ = strconv.ParseFloat(strings.TrimSuffix(fields[i+2], "M"), 64)
		}
	}
	if total > 0 && s.SwapUsedMB == total {
		t.Errorf("swap used (%v) equals total (%v) — the parser is reading the wrong field", s.SwapUsedMB, total)
	}
}

// A row that never measured memory must say so rather than reporting a confident zero,
// which would read as "nothing swapped".
func TestUnmeasuredMemoryIsNotAConfidentZero(t *testing.T) {
	r := NewRow("cfg", 0, "off", "", Sampling{}, ServerProps{}, "toolcall",
		Result{TaskID: "t", MemMeasured: false})
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back["mem_measured"] != false {
		t.Errorf("row does not distinguish unmeasured memory from zero swap: %v", back["mem_measured"])
	}
}

// A backend with no /props must still be scoreable, and its rows must say the served
// config was unavailable rather than report a zero that reads as "0 context".
func TestPropsUnavailableIsRecordedNotFatal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r) // no /props, like an MLX server
	}))
	t.Cleanup(srv.Close)
	p, err := NewClient(srv.URL, 0).Props(context.Background())
	if err == nil && p.Available {
		t.Error("a backend without /props reported its config as available")
	}
	row := NewRow("cfg", 0, "off", "", Sampling{}, ServerProps{}, "toolcall", Result{TaskID: "t"})
	if row.ServedNCtx != 0 {
		t.Errorf("unexpected served ctx: %d", row.ServedNCtx)
	}
}

// The scorer must not be able to set the toggle without the model saying how. A profile that
// names no mechanism is refused at load, so no run can reach a request with the toggle
// silently unset while its row claims a mode.
func TestLoadProfileRefusesAThinkingMechanismItCannotPerform(t *testing.T) {
	dir := t.TempDir()
	body := `{"name":"x","thinking":{"mechanism":"%s","key":"%s"},
	  "sampling":{"thinking":{},"nonthinking":{}},"reasoning":{}}`
	for _, tc := range []struct{ mechanism, key string }{
		{"none", "enable_thinking"},
		{"http_header", "think"},
		{"chat_template_kwarg", ""},
	} {
		path := filepath.Join(dir, "p.json")
		if err := os.WriteFile(path, []byte(fmt.Sprintf(body, tc.mechanism, tc.key)), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadProfile(path); err == nil {
			t.Errorf("mechanism %q key %q loaded; it cannot switch the toggle", tc.mechanism, tc.key)
		}
	}
}

// The toggle is rendered from the profile, not from a constant in the scorer: that is the
// whole of what makes a second model scoreable without editing Go.
func TestThinkingKwargsComesFromTheProfile(t *testing.T) {
	for _, on := range []bool{true, false} {
		kw, err := qwenProfile(t).ThinkingKwargs(on)
		if err != nil {
			t.Fatalf("ThinkingKwargs(%v): %v", on, err)
		}
		if got, want := kw["enable_thinking"], on; got != want {
			t.Errorf("ThinkingKwargs(%v) = %v, want the profile key set to %v", on, kw, want)
		}
		if len(kw) != 1 {
			t.Errorf("ThinkingKwargs sent %d keys, want only the one the profile names: %v", len(kw), kw)
		}
	}
}
