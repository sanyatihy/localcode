package eval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanyatihy/localcode/internal/build"
)

// Two ggml versions can be installed at once and the process opens one of them, so the
// library a row names has to be the one the server has open rather than the one on a
// default path. libggml-base is loaded by every build and carries no kernels.
func TestBackendNamesTheKernelLibraryAndNotTheSharedHalf(t *testing.T) {
	lsof := "p8123\n" +
		"n/opt/homebrew/Cellar/llama.cpp/0.4.0/lib/libllama.0.4.0.dylib\n" +
		"n/opt/homebrew/Cellar/ggml/0.23.0/lib/libggml-base.0.23.0.dylib\n" +
		"n/opt/homebrew/Cellar/ggml/0.23.0/libexec/libggml-metal.so\n"
	if got, want := backendFrom(lsof), "/opt/homebrew/Cellar/ggml/0.23.0/libexec/libggml-metal.so"; got != want {
		t.Errorf("backend %q, want %q", got, want)
	}
}

// A server whose kernels are linked in rather than opened has no such file, and a row
// that named the closest match would attribute its numbers to a library that served none
// of them.
func TestBackendIsUnknownWhenNoKernelLibraryIsOpen(t *testing.T) {
	lsof := "p8123\nn/usr/lib/libobjc-trampolines.dylib\n" +
		"n/opt/homebrew/Cellar/ggml/0.23.0/lib/libggml-base.0.23.0.dylib\n"
	if got := backendFrom(lsof); got != build.Unknown {
		t.Errorf("backend %q, want %q", got, build.Unknown)
	}
}

func TestPortOfReadsTheEndpointOrRefuses(t *testing.T) {
	for _, tc := range []struct{ endpoint, want string }{
		{"http://127.0.0.1:8081", "8081"},
		{"http://127.0.0.1:8081/v1", "8081"},
		{"127.0.0.1:8081", "8081"},
		{"http://localhost", ""},
		{"", ""},
	} {
		if got := portOf(tc.endpoint); got != tc.want {
			t.Errorf("portOf(%q) = %q, want %q", tc.endpoint, got, tc.want)
		}
	}
}

// The kernels and the build are versioned apart, so a row carries both or it cannot say
// what decoded it.
func TestPropsCarriesTheStackThatServedARow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model_path":                  "/hub/models--bartowski--Qwen3.8-27B-GGUF/snapshots/f0eec4a4/Qwen3.8-27B-Q4_K_M.gguf",
			"chat_template":               "{%- set x = 1 %}",
			"build_info":                  "10809-5266f24da",
			"default_generation_settings": map[string]any{"n_ctx": 32768},
		})
	}))
	t.Cleanup(srv.Close)

	restore := Backend
	Backend = func(string) string { return "/opt/homebrew/Cellar/ggml/0.23.0/libexec/libggml-metal.so" }
	t.Cleanup(func() { Backend = restore })

	p, err := NewClient(srv.URL, 0).Props(context.Background())
	if err != nil {
		t.Fatalf("props: %v", err)
	}
	row := NewRow("cfg", 0, "off", "", Sampling{}, p, "toolcall", Result{TaskID: "t"})
	if row.ServedBuild != "10809-5266f24da" {
		t.Errorf("served_build %q", row.ServedBuild)
	}
	if row.ServedBackend != "/opt/homebrew/Cellar/ggml/0.23.0/libexec/libggml-metal.so" {
		t.Errorf("served_backend %q", row.ServedBackend)
	}
	if row.ServedSnapshot != "f0eec4a4" {
		t.Errorf("served_snapshot %q", row.ServedSnapshot)
	}
	if row.ServedTemplate != hashTemplate("{%- set x = 1 %}") {
		t.Errorf("served_template %q", row.ServedTemplate)
	}
}

// An empty string would read as a row written before the field existed. A backend that
// could not be asked is a different fact, and the rows that carry it say so.
func TestRowSaysUnknownWhereTheStackCouldNotBeRead(t *testing.T) {
	row := NewRow("cfg", 0, "off", "", Sampling{}, ServerProps{}, "toolcall", Result{TaskID: "t"})
	for name, got := range map[string]string{"served_build": row.ServedBuild,
		"served_backend": row.ServedBackend, "served_snapshot": row.ServedSnapshot,
		"served_template": row.ServedTemplate} {
		if got != build.Unknown {
			t.Errorf("%s %q, want %q", name, got, build.Unknown)
		}
	}
}

// `-hf` cannot pin a revision and a re-upload keeps the file name, so the snapshot in the
// cache path is the only thing that tells two sets of weights apart.
func TestSnapshotComesFromTheCachePath(t *testing.T) {
	const cached = "/Users/x/.cache/huggingface/hub/models--bartowski--Qwen3.8-27B-GGUF/" +
		"snapshots/f0eec4a4bb4975114a030d048952d83c0a53c034/Qwen3.8-27B-Q4_K_M.gguf"
	if got, want := snapshotOf(cached), "f0eec4a4bb4975114a030d048952d83c0a53c034"; got != want {
		t.Errorf("snapshot %q, want %q", got, want)
	}
	if got := snapshotOf("/models/local.gguf"); got != build.Unknown {
		t.Errorf("snapshot %q, want %q for a path with no revision in it", got, build.Unknown)
	}
}

// A config that names a template the server declined to load renders the model's own, and
// a path would record the request rather than what was served.
func TestTemplateIsHashedFromWhatTheServerHolds(t *testing.T) {
	one, two := hashTemplate("{{ system }}"), hashTemplate("{{ system }} ")
	if one == two {
		t.Error("two templates hashed alike")
	}
	if len(one) != 12 {
		t.Errorf("hash %q, want twelve characters", one)
	}
	if got := hashTemplate(""); got != build.Unknown {
		t.Errorf("hash %q, want %q where the server reported no template", got, build.Unknown)
	}
}
