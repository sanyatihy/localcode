package eval

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A runtime with no /props says what it serves on /status and /v1/models (0062: Splash). Its
// rows then name a context, a model and a build where they carried 0 and "." before, and its
// Messages requests ask for the one model name it answers to.
func TestPropsReadsStatusWhereThereIsNoProps(t *testing.T) {
	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status":
			_, _ = io.WriteString(w, `{"ready":true,"maximum_context_tokens":49152,"identity":{"cache":{"build_id":"src-abc"}}}`)
		case "/v1/models":
			_, _ = io.WriteString(w, `{"data":[{"id":"vendor/model-package"}]}`)
		case "/v1/messages":
			var body struct {
				Model string `json:"model"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			asked = body.Model
			_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":1}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 5*time.Second)
	c.API = APIMessages
	p, err := c.Props(context.Background())
	if err != nil {
		t.Fatalf("props: %v", err)
	}
	if !p.Available || p.NCtx != 49152 || p.ModelPath != "vendor/model-package" || p.BuildInfo != "src-abc" {
		t.Errorf("props off /status: %+v", p)
	}
	if _, err := c.Converse(context.Background(), []Message{{Role: "user", Content: "hi"}}, 8); err != nil {
		t.Fatalf("converse: %v", err)
	}
	if asked != "vendor/model-package" {
		t.Errorf("a server that names its model must be asked by that name, got %q", asked)
	}
}

// Only a 404 sends Props to /status, and a /status that is not ready or names no model is
// not an answer: a row must say "unavailable" before it says a context nothing is serving.
func TestPropsRefusesAStatusItCannotUse(t *testing.T) {
	cases := map[string]struct{ status, models string }{
		"not ready": {`{"ready":false,"maximum_context_tokens":49152}`, `{"data":[{"id":"m"}]}`},
		"no model":  {`{"ready":true,"maximum_context_tokens":49152}`, `{"data":[]}`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/status":
					_, _ = io.WriteString(w, c.status)
				case "/v1/models":
					_, _ = io.WriteString(w, c.models)
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			if p, err := NewClient(srv.URL, 5*time.Second).Props(context.Background()); err == nil {
				t.Errorf("want an error, got %+v", p)
			}
		})
	}

	var statusAsked bool
	llama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			statusAsked = true
		}
		_, _ = io.WriteString(w, `{"model_path":"/m/x.gguf","default_generation_settings":{"n_ctx":32768}}`)
	}))
	defer llama.Close()
	c := NewClient(llama.URL, 5*time.Second)
	p, err := c.Props(context.Background())
	if err != nil || p.NCtx != 32768 || statusAsked || c.Model != "" {
		t.Errorf("llama.cpp must be read off /props alone: %+v err %v statusAsked %v model %q", p, err, statusAsked, c.Model)
	}
}
