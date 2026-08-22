package main

import (
	"strings"
	"testing"
)

// The stream as the harness really emits it, taken from a captured session: an init, a
// turn that calls a tool, the result, two status rows, the answer, and a result row.
const stream = `{"type":"system","subtype":"init","cwd":"/repo"}
{"type":"stream_event","event":{"type":"content_block_start","content_block":{"type":"text","text":""}}}
{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Fixing the "}}}
{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"even case."}}}
{"type":"stream_event","event":{"type":"content_block_stop"}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/repo/median.go"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"package fixme\n"}]}}
{"type":"system","subtype":"status"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./...\nsecond line"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":[{"type":"text","text":"PreToolUse:Bash hook error: localcode: this session's context reached 7231 tokens of a 6400 ceiling. No call other than writing the handoff will be permitted."}]}]}}
{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Fixed the even-length case."}}}
{"type":"stream_event","event":{"type":"content_block_stop"}}
{"type":"assistant","message":{"content":[{"type":"text","text":"Fixed the even-length case."}]}}
{"type":"result","subtype":"success"}`

func TestProgressShowsEachCallAsItHappens(t *testing.T) {
	var out strings.Builder
	render(strings.NewReader(stream), &out, "/repo")
	got := out.String()

	// The prose arrives a token at a time and has to come out whole, on its own line.
	if !strings.Contains(got, "Fixing the even case.\n") {
		t.Fatalf("streamed text must be joined and closed:\n%q", got)
	}
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
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"after.go"}}]}}` + "\n"
	var out strings.Builder
	render(strings.NewReader(in), &out, "")
	if !strings.Contains(out.String(), "→ Read after.go") {
		t.Fatalf("the stream stopped at the big result:\n%s", out.String())
	}
}

// A tool call must never be printed onto the end of a half-written line of prose.
func TestProgressClosesAStreamedLineBeforeAnythingElse(t *testing.T) {
	in := `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Fixing it now:"}}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/repo/a.go"}}]}}`
	var out strings.Builder
	render(strings.NewReader(in), &out, "/repo")
	if !strings.Contains(out.String(), "Fixing it now:\n  → Edit a.go") {
		t.Fatalf("got %q", out.String())
	}
}

// The complete message repeats text that was already streamed, and printing both doubles it.
func TestProgressDoesNotPrintStreamedTextTwice(t *testing.T) {
	in := `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"done"}}}
{"type":"stream_event","event":{"type":"content_block_stop"}}
{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}}`
	var out strings.Builder
	render(strings.NewReader(in), &out, "")
	if n := strings.Count(out.String(), "done"); n != 1 {
		t.Fatalf("text printed %d times: %q", n, out.String())
	}
}

// A harness that sends no partials must still be heard: the complete message is the
// fallback, and going silent would be the failure this whole thing removes.
func TestProgressStillPrintsTextWhenNothingWasStreamed(t *testing.T) {
	in := `{"type":"assistant","message":{"content":[{"type":"text","text":"no partials here"}]}}`
	var out strings.Builder
	render(strings.NewReader(in), &out, "")
	if !strings.Contains(out.String(), "no partials here") {
		t.Fatalf("got %q", out.String())
	}
}
