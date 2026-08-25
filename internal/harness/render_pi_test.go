package harness

import (
	"strings"
	"testing"
)

// What a person watching a Pi session sees: the model's prose as it arrives, each call it
// makes, and the gate's refusals — the same three the incumbent's stream renders.
func TestPiRenderShowsProseCallsAndRefusals(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"session","version":3,"id":"x","cwd":"/repo"}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Reading "}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"the file."}}`,
		`{"type":"message_end","message":{"role":"assistant"}}`,
		`{"type":"tool_execution_start","toolCallId":"1","toolName":"read","args":{"path":"/repo/median.go"}}`,
		`{"type":"tool_execution_end","toolCallId":"1","toolName":"read","isError":false,` +
			`"result":{"content":[{"type":"text","text":"package main"}]}}`,
		`{"type":"tool_execution_start","toolCallId":"2","toolName":"bash","args":{"command":"go test ./..."}}`,
		`{"type":"tool_execution_end","toolCallId":"2","toolName":"bash","isError":true,` +
			`"result":{"content":[{"type":"text","text":"localcode: this session has spent 30 of 30 tool calls."}]}}`,
	}, "\n") + "\n"

	var out strings.Builder
	if overrun := renderPiJSON(strings.NewReader(stream), &out, "/repo"); overrun != "" {
		t.Errorf("no prompt was refused, so nothing must be reported as one: %q", overrun)
	}
	for _, want := range []string{
		"Reading the file.",          // streamed a token at a time, printed as one line
		"  → read median.go",         // the path relative to the repository being worked in
		"  → bash go test ./...",     // the command, which is what a bash call is
		"  ✗ this session has spent", // the gate's own words, and only the gate's
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the stream does not show %q:\n%s", want, out.String())
		}
	}
	// A permitted call's result is not news: the session's own words are.
	if strings.Contains(out.String(), "package main") {
		t.Errorf("a tool result must not be printed:\n%s", out.String())
	}
}

// The one failure the budget exists to prevent, and the stream is where it appears: the
// harness surfaces the server's body verbatim and stops.
func TestPiRenderReportsTheServersRefusal(t *testing.T) {
	line := `{"type":"message_end","message":{"role":"assistant","stopReason":"error",` +
		`"errorMessage":"request (49509 tokens) exceeds the available context size (49152 tokens)"}}`
	var out strings.Builder
	overrun := renderPiJSON(strings.NewReader(line+"\n"), &out, "")
	if !strings.Contains(overrun, "49509") || !strings.Contains(overrun, "49152") {
		t.Fatalf("the refusal must carry both numbers: %q", overrun)
	}
	if !strings.Contains(out.String(), "the server refused the prompt") {
		t.Errorf("a session that died of it must say so:\n%s", out.String())
	}
}

// A line this does not model is the harness saying something, and swallowing it is how a
// run goes quiet for a reason nobody can see.
func TestPiRenderPassesThroughWhatItCannotParse(t *testing.T) {
	var out strings.Builder
	renderPiJSON(strings.NewReader("Warning: something happened\n"), &out, "")
	if !strings.Contains(out.String(), "Warning: something happened") {
		t.Errorf("an unparsed line must still be seen:\n%s", out.String())
	}
}
