package harness

import (
	"slices"
	"testing"
)

// A server that answers 404 to any model name but its own has to be asked by that name on
// every variable the harness reads one from (0060).
func TestServedModelNamesEveryModelVariable(t *testing.T) {
	c := &claudeCodeAgent{claudeCodeRecorder: claudeCodeRecorder{env: []string{
		"ANTHROPIC_MODEL=file.gguf", "ANTHROPIC_DEFAULT_HAIKU_MODEL=file.gguf", "OTHER=kept"}}}
	c.ServedModel("vendor/model-package")
	for _, want := range []string{"ANTHROPIC_MODEL=vendor/model-package",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL=vendor/model-package",
		"ANTHROPIC_DEFAULT_SONNET_MODEL=vendor/model-package",
		"ANTHROPIC_DEFAULT_OPUS_MODEL=vendor/model-package", "OTHER=kept"} {
		if !slices.Contains(c.env, want) {
			t.Errorf("env lacks %q: %v", want, c.env)
		}
	}
}
