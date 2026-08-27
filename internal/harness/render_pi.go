package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// renderPiJSON turns Pi's `--mode json` event stream into the same lines the incumbent's
// stream renders to. Both exist for one reason: at this machine's rate a session spends
// minutes between saying anything, and a run that prints nothing is indistinguishable from
// a wedged one. The shapes differ — Pi sends deltas without the cumulative message, and
// reports a tool call as an execution rather than as a content block — so the parsing is
// its own and the presentation is shared.
func renderPiJSON(events io.Reader, out io.Writer, cwd string) (overrun string) {
	scan := newEventScanner(events)
	open := false
	for scan.Scan() {
		line := scan.Bytes()
		if overrun == "" {
			if i := bytes.Index(line, []byte(overrunMarker)); i >= 0 {
				overrun = strings.TrimSpace(errorSentence(string(line), i))
			}
		}
		var row struct {
			Type     string `json:"type"`
			ToolName string `json:"toolName"`
			Args     map[string]any
			Result   json.RawMessage `json:"result"`
			Event    struct {
				Type  string `json:"type"`
				Delta string `json:"delta"`
			} `json:"assistantMessageEvent"`
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
		switch row.Type {
		case "message_update":
			// The model's prose a token at a time, which is the whole difference between
			// watching a session and waiting on one. Pi sends the delta alone, so what is
			// printed is what arrived.
			switch row.Event.Type {
			case "text_delta":
				_, _ = io.WriteString(out, row.Event.Delta)
				open = true
			}
		case "message_end":
			open = closeLine(out, open)
		case "tool_execution_start":
			open = closeLine(out, open)
			say(out, fmt.Sprintf("  → %s %s", row.ToolName, argOf(row.ToolName, row.Args, cwd)))
		case "tool_execution_end":
			if reason := refusal(row.Result); reason != "" {
				open = closeLine(out, open)
				say(out, "  ✗ "+reason)
			}
		}
	}
	finish(scan, events, out, open)
	if overrun != "" {
		_ = closeLine(out, open)
		say(out, "  ✗ the server refused the prompt: "+overrun)
	}
	return overrun
}
