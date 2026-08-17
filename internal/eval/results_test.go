package eval

import "testing"

// Tool-call validity is a derived metric, and the derivation is the whole point:
// a parseable call to the wrong tool is a reasoning failure, an unparseable one is
// a format failure, and a server error is neither. Conflating them would send 0005
// after the wrong lever.
func TestToolCallValid(t *testing.T) {
	tests := []struct {
		kind           string
		outcome        Outcome
		valid, applies bool
	}{
		{"toolcall", Pass, true, true},
		{"toolcall", FailWrongTool, true, true}, // valid JSON, wrong choice
		{"toolcall", FailArgs, true, true},      // valid JSON, wrong contents
		{"toolcall", FailBadJSON, false, true},  // the format failure
		{"toolcall", FailNoCall, false, true},   // prose instead of a call
		{"toolcall", FailServer, false, false},  // not the model's doing
		{"patch", Pass, false, false},           // metric does not apply
		{"retrieval", FailRetrieval, false, false},
	}
	for _, tc := range tests {
		r := Row{Kind: tc.kind, Outcome: tc.outcome}
		valid, applies := r.ToolCallValid()
		if valid != tc.valid || applies != tc.applies {
			t.Errorf("%s/%s: got (valid=%v, applicable=%v), want (%v, %v)",
				tc.kind, tc.outcome, valid, applies, tc.valid, tc.applies)
		}
	}
}
