package harness

import (
	"io"
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
	renderStreamJSON(strings.NewReader(stream), &out, "/repo")
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
	renderStreamJSON(strings.NewReader("Warning: no stdin data received in 3s\n{\"type\":\"result\"}\n"), &out, "")
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
	renderStreamJSON(strings.NewReader(in), &out, "")
	if !strings.Contains(out.String(), "→ Read after.go") {
		t.Fatalf("the stream stopped at the big result:\n%s", out.String())
	}
}

// Past the buffer a scanner stops the way it stops at the end of a stream, so a render
// that does not ask reports a truncated session as a complete one. Asserted at the bound
// rather than under it: the test above passes at 300 KB whether the bound is guarded or
// not, which is how it went unguarded.
func TestARenderPastItsBufferSaysSoRatherThanGoingQuiet(t *testing.T) {
	line := `{"type":"user","message":{"content":[{"type":"tool_result","content":"` +
		strings.Repeat("x", eventBuffer+1) + `"}]}}`
	for _, r := range []struct {
		name   string
		render func(io.Reader, io.Writer, string) string
	}{
		{"claude-code", renderStreamJSON},
		{"pi", renderPiJSON},
	} {
		t.Run(r.name, func(t *testing.T) {
			var out strings.Builder
			r.render(strings.NewReader(line+"\ntrailing\n"), &out, "")
			if !strings.Contains(out.String(), "could not be read past") {
				t.Fatalf("a stream this could not read must say so: %q", out.String())
			}
		})
	}
}

// The caller reads this off a pipe and then waits on the process, so a render that stops
// reading leaves the child blocked in its own write and the session dies on the
// supervisor's clock instead of its exit code.
func TestARenderPastItsBufferDrainsWhatItWillNotShow(t *testing.T) {
	rest := strings.Repeat("y", 64*1024)
	events := strings.NewReader(strings.Repeat("x", eventBuffer+1) + "\n" + rest)
	var out strings.Builder
	renderStreamJSON(events, &out, "")
	if events.Len() != 0 {
		t.Fatalf("%d bytes left unread: a reader that stops wedges the process it reads from",
			events.Len())
	}
}

// A tool call must never be printed onto the end of a half-written line of prose.
func TestProgressClosesAStreamedLineBeforeAnythingElse(t *testing.T) {
	in := `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Fixing it now:"}}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/repo/a.go"}}]}}`
	var out strings.Builder
	renderStreamJSON(strings.NewReader(in), &out, "/repo")
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
	renderStreamJSON(strings.NewReader(in), &out, "")
	if n := strings.Count(out.String(), "done"); n != 1 {
		t.Fatalf("text printed %d times: %q", n, out.String())
	}
}

// A harness that sends no partials must still be heard: the complete message is the
// fallback, and going silent would be the failure this whole thing removes.
func TestProgressStillPrintsTextWhenNothingWasStreamed(t *testing.T) {
	in := `{"type":"assistant","message":{"content":[{"type":"text","text":"no partials here"}]}}`
	var out strings.Builder
	renderStreamJSON(strings.NewReader(in), &out, "")
	if !strings.Contains(out.String(), "no partials here") {
		t.Fatalf("got %q", out.String())
	}
}

// The one failure the budget exists to prevent. The harness surfaces the server's body and
// stops, so without this a session that overran ends with an exit code and nothing that
// says why — and the sentence carries both numbers, which is the measurement of by how much.
func TestProgressReportsTheServerRefusingThePrompt(t *testing.T) {
	const line = `{"type":"result","is_error":true,"result":"API Error: 400 request ` +
		`(49509 tokens) exceeds the available context size (49152 tokens), try increasing it"}`
	var out strings.Builder
	overrun := renderStreamJSON(strings.NewReader(line+"\n"), &out, "")
	if !strings.Contains(overrun, "49509 tokens") || !strings.Contains(overrun, "49152 tokens") {
		t.Fatalf("the server's own numbers must survive: %q", overrun)
	}
	if strings.ContainsAny(overrun, `{}"`) {
		t.Fatalf("the JSON around the sentence must not: %q", overrun)
	}
	if !strings.Contains(out.String(), "the server refused the prompt") {
		t.Fatalf("it must be said as it happens:\n%s", out.String())
	}
}

// And a session that did not overrun says nothing, because an empty string is what the
// chain records as "this did not happen".
func TestProgressReportsNoOverrunWhenThereWasNone(t *testing.T) {
	var out strings.Builder
	if overrun := renderStreamJSON(strings.NewReader(stream), &out, "/repo"); overrun != "" {
		t.Fatalf("a session that fit must report nothing: %q", overrun)
	}
}
