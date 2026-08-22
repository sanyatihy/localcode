package main

import (
	"strings"
	"testing"
)

// The stream as the harness really emits it, taken from a captured session: an init, a
// turn that calls a tool, the result, two status rows, the answer, and a result row.
const stream = `{"type":"system","subtype":"init","cwd":"/repo"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/repo/median.go"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"package fixme\n"}]}}
{"type":"system","subtype":"status"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./...\nsecond line"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":[{"type":"text","text":"PreToolUse:Bash hook error: localcode: this session's context reached 7231 tokens of a 6400 ceiling. No call other than writing the handoff will be permitted."}]}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"Fixed the even-length case."}]}}
{"type":"result","subtype":"success"}`

func TestProgressShowsEachCallAsItHappens(t *testing.T) {
	var out strings.Builder
	render(strings.NewReader(stream), &out, "/repo")
	got := out.String()

	for _, want := range []string{"→ Read median.go", "→ Bash go test ./...", "Fixed the even-length case."} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	// A command spanning lines must not spill over the one line it is given.
	if strings.Contains(got, "second line") {
		t.Fatalf("a multi-line command must be cut to one line:\n%s", got)
	}
	// The gate refusing is the mechanism working, and it is the one result worth a line.
	if !strings.Contains(got, "✗ this session's context reached 7231") {
		t.Fatalf("a denial must be shown:\n%s", got)
	}
	// The tool result itself is not: it is what fills the window, not what explains it.
	if strings.Contains(got, "package fixme") {
		t.Fatalf("tool output must not be echoed:\n%s", got)
	}
}

// An unparsed line is the harness saying something. Swallowing it is how a run goes quiet
// for a reason nobody can see — which is the failure this whole thing removes.
func TestProgressPassesThroughWhatItCannotParse(t *testing.T) {
	var out strings.Builder
	render(strings.NewReader("Warning: no stdin data received in 3s\n{\"type\":\"result\"}\n"), &out, "")
	if !strings.Contains(out.String(), "no stdin data") {
		t.Fatalf("got %q", out.String())
	}
}

// A tool result is a whole file, and the default scanner buffer would end the stream at the
// first big one without saying so.
func TestProgressSurvivesAResultBiggerThanAScannerBuffer(t *testing.T) {
	big := strings.Repeat("x", 300*1024)
	in := `{"type":"user","message":{"content":[{"type":"tool_result","content":"` + big + `"}]}}` + "\n" +
		`{"type":"assistant","message":{"content":[{"type":"text","text":"after the big one"}]}}` + "\n"
	var out strings.Builder
	render(strings.NewReader(in), &out, "")
	if !strings.Contains(out.String(), "after the big one") {
		t.Fatalf("the stream stopped at the big result:\n%s", out.String())
	}
}
