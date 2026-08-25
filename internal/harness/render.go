package harness

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// overrunMarker is what llama-server says when a prompt does not fit the context it serves:
// `request (49509 tokens) exceeds the available context size (49152 tokens), try increasing
// it`. Measured against build 10450, with the harness's own check off so the request
// reaches the server at all.
const overrunMarker = "exceeds the available context size"

// errorSentence cuts the server's sentence out of whatever the harness wrapped it in. The
// numbers either side of the marker are the whole value of the line, so it keeps them and
// drops the JSON around them.
func errorSentence(line string, at int) string {
	start := strings.LastIndexAny(line[:at], "\"\n") + 1
	end := strings.IndexAny(line[at:], "\"\n")
	if end < 0 {
		return line[start:]
	}
	return line[start : at+end]
}

// say writes one line. A terminal that has gone away is not something a session can act
// on, and stopping the run to report it would end the work this exists to watch.
func say(out io.Writer, line string) {
	_, _ = fmt.Fprintln(out, line)
}

// closeLine ends a streamed line before anything else is written on it, and reports that
// there is no longer one open.
func closeLine(out io.Writer, open bool) bool {
	if open {
		say(out, "")
	}
	return false
}

// argOf is the one argument worth showing for a call. Tool names are matched without case
// because the agents spell theirs differently — `Bash` against `bash` — and what is worth
// showing about a call is the same either way. The same choice session-end.sh makes
// for a handoff, for the same reason: a tool name alone does not say what is happening.
func argOf(tool string, input map[string]any, cwd string) string {
	if strings.EqualFold(tool, "bash") {
		return oneLine(str(input["command"]), 70)
	}
	for _, key := range []string{"file_path", "path", "notebook_path", "pattern", "url"} {
		if v := str(input[key]); v != "" {
			if strings.HasSuffix(key, "path") {
				v = shortPath(v, cwd)
			}
			return oneLine(v, 70)
		}
	}
	return ""
}

// refusal picks the gate's own denial out of a tool result, and returns "" for anything
// else. A denied call is the mechanism working, so it is the one result worth a line.
func refusal(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Three shapes for one thing, because the agents wrap a result differently: a bare
	// string, the blocks themselves, or an object carrying them.
	type block struct {
		Text string `json:"text"`
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		var blocks []block
		if json.Unmarshal(raw, &blocks) != nil {
			var wrapped struct {
				Content []block `json:"content"`
			}
			if json.Unmarshal(raw, &wrapped) != nil {
				return ""
			}
			blocks = wrapped.Content
		}
		for _, b := range blocks {
			text += b.Text
		}
	}
	_, after, found := strings.Cut(text, "localcode: ")
	if !found {
		return ""
	}
	return oneLine(after, 100)
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func shortPath(path, cwd string) string {
	if cwd == "" || !filepath.IsAbs(path) {
		return path
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.Base(path)
	}
	return rel
}

func oneLine(s string, width int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > width {
		return s[:width] + "…"
	}
	return s
}
