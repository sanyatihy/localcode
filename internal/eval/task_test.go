package eval

import "testing"

// The checker is the instrument every number in this project rests on, so its
// failure paths are tested offline rather than only exercised against a server
// that happens to behave. A check that cannot fail is checking nothing.
func TestCheckToolCall(t *testing.T) {
	exp := Expect{
		Tool:         "read_file",
		RequiredArgs: []string{"path"},
		ArgContains:  map[string]string{"path": "session.go"},
	}
	call := func(name, args string) []ToolCall {
		var c ToolCall
		c.Function.Name = name
		c.Function.Arguments = args
		return []ToolCall{c}
	}

	tests := []struct {
		name  string
		calls []ToolCall
		text  string
		want  Outcome
	}{
		{"correct call", call("read_file", `{"path":"src/auth/session.go"}`), "", Pass},
		{"extra args are fine", call("read_file", `{"path":"session.go","start_line":1}`), "", Pass},
		{"prose instead of a call", nil, "I would read the file.", FailNoCall},
		{"wrong tool", call("edit_file", `{"path":"session.go"}`), "", FailWrongTool},
		{"truncated json", call("read_file", `{"path":"session.go`), "", FailBadJSON},
		{"not json at all", call("read_file", `path=session.go`), "", FailBadJSON},
		{"missing required arg", call("read_file", `{"line":3}`), "", FailArgs},
		{"wrong arg value", call("read_file", `{"path":"src/main.go"}`), "", FailArgs},
		{"arg wrong type", call("read_file", `{"path":42}`), "", FailArgs},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, detail := checkToolCall(exp, tc.calls, tc.text)
			if got != tc.want {
				t.Errorf("got %s (%s), want %s", got, detail, tc.want)
			}
		})
	}
}
