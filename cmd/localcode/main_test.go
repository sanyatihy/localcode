package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeCheckout builds the one file the launcher reads out of a checkout, so a test never
// depends on the real one being where the test runner happens to stand.
func fakeCheckout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "harness", "claude-code")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	env := `ANTHROPIC_BASE_URL="http://127.0.0.1:8081"` + "\n" + `ANTHROPIC_AUTH_TOKEN="local"` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "claude-code.env"), []byte(env), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// stubClaude puts a recording `claude` on PATH, so the launcher can be tested without a
// model, a server, or the several minutes a 27B answer costs.
func stubClaude(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	argv := filepath.Join(dir, "argv")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argv + "\n" + script + "\n"
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argv
}

func healthy(t *testing.T, code int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// The property the whole feature rests on: a session leaves the repository it visited
// exactly as it found it. Asserted against the launcher, which is the part this repo owns.
func TestRunWritesNothingToTheRepository(t *testing.T) {
	root := fakeCheckout(t)
	stubClaude(t, "exit 0")
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "only.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	if code, err := run(opts{checkout: root, endpoint: healthy(t, http.StatusOK), noServe: true}); err != nil || code != 0 {
		t.Fatalf("run: code %d, err %v", code, err)
	}

	entries, err := os.ReadDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "only.txt" {
		var got []string
		for _, e := range entries {
			got = append(got, e.Name())
		}
		t.Fatalf("the repository was written to: %v", got)
	}
}

// --tools without --allowedTools is a session that stops to ask, and an answer given once
// is recorded in the visited repository. They travel together or the property above breaks.
func TestRunPreapprovesTheToolsItExposes(t *testing.T) {
	root := fakeCheckout(t)
	argv := stubClaude(t, "exit 0")
	t.Chdir(t.TempDir())

	if code, err := run(opts{checkout: root, endpoint: healthy(t, http.StatusOK), noServe: true}); err != nil || code != 0 {
		t.Fatalf("run: code %d, err %v", code, err)
	}
	got, err := os.ReadFile(argv)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Fields(string(got))
	tools := flagValue(args, "--tools")
	allowed := flagValue(args, "--allowedTools")
	if tools == "" || tools != allowed {
		t.Fatalf("--tools %q and --allowedTools %q must name the same set", tools, allowed)
	}
}

// With no prompt the session is interactive; -p would make it answer once and exit.
func TestRunIsInteractiveWithoutAPrompt(t *testing.T) {
	root := fakeCheckout(t)
	argv := stubClaude(t, "exit 0")
	t.Chdir(t.TempDir())

	if _, err := run(opts{checkout: root, endpoint: healthy(t, http.StatusOK), noServe: true}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(argv)
	if strings.Contains(string(got), "-p") {
		t.Fatalf("no prompt was given, so the session must not be -p: %s", got)
	}
}

func TestRunRefusesWhenTheServerIsNotReady(t *testing.T) {
	root := fakeCheckout(t)
	stubClaude(t, "exit 0")
	t.Chdir(t.TempDir())

	// 503 is what llama-server answers while it loads: something is listening, and it
	// cannot serve yet.
	code, err := run(opts{checkout: root, endpoint: healthy(t, http.StatusServiceUnavailable), noServe: true})
	if code != 2 || err == nil {
		t.Fatalf("a loading server must refuse, got code %d err %v", code, err)
	}
}

func TestRunPropagatesTheAgentsExitCode(t *testing.T) {
	root := fakeCheckout(t)
	stubClaude(t, "exit 3")
	t.Chdir(t.TempDir())

	code, err := run(opts{checkout: root, endpoint: healthy(t, http.StatusOK), noServe: true})
	if err != nil || code != 3 {
		t.Fatalf("want exit 3 passed through, got %d err %v", code, err)
	}
}

// -no-serve is what a script wants: it must refuse rather than spend twenty seconds and
// most of the machine's memory on the caller's behalf.
func TestRunRefusesRatherThanServingWhenToldNotTo(t *testing.T) {
	root := fakeCheckout(t)
	stubClaude(t, "exit 0")
	t.Chdir(t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening now

	code, err := run(opts{checkout: root, endpoint: url, noServe: true})
	if code != 2 || err == nil {
		t.Fatalf("want a refusal, got code %d err %v", code, err)
	}
	if !strings.Contains(err.Error(), "no-serve") {
		t.Fatalf("the refusal must say which flag caused it: %v", err)
	}
}

func TestResolveCheckoutRefusesSomethingThatIsNotOne(t *testing.T) {
	if _, err := resolveCheckout(t.TempDir()); err == nil {
		t.Fatal("an empty directory is not a checkout and must be refused")
	}
	if _, err := resolveCheckout(""); err == nil {
		t.Fatal("no checkout at all must be refused, not guessed")
	}
}

func flagValue(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestStatusReportsWhatIsServed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model_path":"/cache/Qwen3.8-27B-Q4_K_M.gguf",
			"default_generation_settings":{"n_ctx":49152}}`))
	}))
	defer srv.Close()

	code, err := status(srv.URL)
	if err != nil || code != 0 {
		t.Fatalf("a serving endpoint must report 0: code %d err %v", code, err)
	}
}

// Down is an answer, not a failure to run: a script asking "is it up" gets 1, and 2 stays
// reserved for the cases where the question could not be put.
func TestStatusAnswersNoWhenNothingIsServing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	code, err := status(url)
	if err != nil || code != 1 {
		t.Fatalf("a dead endpoint must answer 1 with no error: code %d err %v", code, err)
	}
}

// serve and stop must reach the checkout's own scripts, not a second implementation:
// stop.sh waits for ~17 GB to be released, and a launcher that re-solved that would be
// the fifth home for one fact.
func TestServeAndStopRunTheCheckoutsOwnScripts(t *testing.T) {
	root := fakeCheckout(t)
	dir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "ran")
	for _, name := range []string{"serve.sh", "stop.sh"} {
		body := "#!/bin/sh\necho " + name + " \"$@\" >> " + marker + "\nexit 0\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if code, err := script(root, "serve.sh", "config/agent.env"); err != nil || code != 0 {
		t.Fatalf("serve: code %d err %v", code, err)
	}
	if code, err := script(root, "stop.sh"); err != nil || code != 0 {
		t.Fatalf("stop: code %d err %v", code, err)
	}

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "serve.sh config/agent.env") {
		t.Fatalf("serve.sh must receive the config: %s", got)
	}
	if !strings.Contains(string(got), "stop.sh") {
		t.Fatalf("stop.sh must have run: %s", got)
	}
}

func TestScriptPassesTheExitCodeThrough(t *testing.T) {
	root := fakeCheckout(t)
	dir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stop.sh"), []byte("#!/bin/sh\nexit 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code, err := script(root, "stop.sh"); err != nil || code != 2 {
		t.Fatalf("want 2 passed through, got %d err %v", code, err)
	}
}
