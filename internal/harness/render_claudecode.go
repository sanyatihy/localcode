package harness

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// renderStreamJSON turns Claude Code's `--output-format stream-json` into what a person
// watching wants to see.
//
// `claude -p` prints its result and nothing before it, which on this machine is minutes of
// silence: one measured session spent 591 s deciding before it said anything, and a session
// at the shipped window is longer. Nothing distinguishes that from a wedged run. Asking for
// the events instead means the supervisor owns every line the developer sees, so this
// prints the model's own words as well as what it is doing.
//
// It returns the server's refusal when one goes past, because that is the one failure the
// budget exists to prevent and the stream is where it appears: the harness surfaces the
// body verbatim and stops, so a session that dies of it otherwise ends with an exit code
// and nothing that says why.
func renderStreamJSON(events io.Reader, out io.Writer, cwd string) (overrun string) {
	scan := bufio.NewScanner(events)
	// A line here carries a whole tool result. The default 64 KB would end the stream at the
	// first big one, and silently — which is the failure this exists to remove.
	scan.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	// Whether a half-written line of the model's prose is still open, so nothing else is
	// printed onto the end of it, and whether this message's text has already been
	// streamed a token at a time.
	open, streamed := false, false
	for scan.Scan() {
		line := scan.Bytes()
		// llama-server's own wording, and it names both numbers — the request and the
		// context it did not fit. Matched on the raw line because it reaches the stream in
		// whatever shape the harness wraps an error in, and the sentence is the server's
		// either way.
		if overrun == "" {
			if i := bytes.Index(line, []byte(overrunMarker)); i >= 0 {
				overrun = strings.TrimSpace(errorSentence(string(line), i))
			}
		}
		var row struct {
			Type  string `json:"type"`
			Event struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			} `json:"event"`
			Message struct {
				Content []struct {
					Type    string          `json:"type"`
					Text    string          `json:"text"`
					Name    string          `json:"name"`
					Input   map[string]any  `json:"input"`
					Content json.RawMessage `json:"content"`
				} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(line, &row); err != nil {
			// Not an event this models. Passed through rather than dropped: an unparsed
			// line is the harness saying something, and swallowing it is how a run goes
			// quiet for a reason nobody can see.
			if s := strings.TrimSpace(string(line)); s != "" {
				open = closeLine(out, open)
				say(out, s)
			}
			continue
		}
		// The model's prose arrives a token at a time, which is the whole difference
		// between watching a session and waiting on one: at 5-10 tok/s a paragraph is a
		// minute, and a minute of nothing is indistinguishable from a wedged run.
		if row.Type == "stream_event" {
			switch {
			case row.Event.Type == "content_block_delta" && row.Event.Delta.Type == "text_delta":
				_, _ = io.WriteString(out, row.Event.Delta.Text)
				open, streamed = true, true
			case row.Event.Type == "content_block_stop" && open:
				say(out, "")
				open = false
			}
			continue
		}

		for _, b := range row.Message.Content {
			switch b.Type {
			case "text":
				// The complete message repeats what was streamed, so it is printed only
				// when nothing was — a harness that sends no partials must still be heard.
				if !streamed {
					if s := strings.TrimSpace(b.Text); s != "" {
						say(out, s)
					}
				}
			case "tool_use":
				open = closeLine(out, open)
				say(out, fmt.Sprintf("  → %s %s", b.Name, argOf(b.Name, b.Input, cwd)))
			case "tool_result":
				if reason := refusal(b.Content); reason != "" {
					open = closeLine(out, open)
					say(out, "  ✗ "+reason)
				}
			}
		}
		if row.Type == "assistant" {
			streamed = false
		}
	}
	if overrun != "" {
		_ = closeLine(out, open)
		say(out, "  ✗ the server refused the prompt: "+overrun)
	}
	return overrun
}
