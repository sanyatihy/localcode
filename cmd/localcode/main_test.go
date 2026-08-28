package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sanyatihy/localcode/internal/chain"
)

// fakeCheckout builds the one file the launcher reads out of a checkout, so a test never
// depends on the real one being where the test runner happens to stand.
func fakeCheckout(t *testing.T) string {
	t.Helper()
	// A home of its own: a session's state directory hangs off it, and a test that wrote
	// into the developer's would leave chains behind that `localcode sessions` lists.
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	dir := filepath.Join(root, "harness", "claude-code")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The two window variables are here because the launcher refuses without them: a
	// session's budget is derived from the window the harness was declared.
	env := `ANTHROPIC_BASE_URL="http://127.0.0.1:8081"` + "\n" +
		`ANTHROPIC_AUTH_TOKEN="local"` + "\n" +
		`CLAUDE_CODE_MAX_CONTEXT_TOKENS="45056"` + "\n" +
		`CLAUDE_CODE_MAX_OUTPUT_TOKENS="4096"` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "claude-code.env"), []byte(env), 0o644); err != nil {
		t.Fatal(err)
	}
	// What says a directory is a checkout at all, and it is not any harness's file: the
	// launcher starts the server from here.
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "serve.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// stubAgent puts a recording binary of that name on PATH, so the launcher can be tested
// without a model, a server, or the several minutes a 27B answer costs.
func stubAgent(t *testing.T, name, script string) string {
	t.Helper()
	dir := t.TempDir()
	argv := filepath.Join(dir, "argv")
	// And its environment beside them: half of what the launcher decides reaches the
	// session as a variable rather than as a flag.
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argv + "\nenv > " + argv + ".env\n" +
		script + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argv
}

// stubClaude puts a recording `claude` on PATH, so the launcher can be tested without a
// model, a server, or the several minutes a 27B answer costs.
func stubClaude(t *testing.T, script string) string {
	t.Helper()
	return stubAgent(t, "claude", script)
}

// passthroughSandbox substitutes a stand-in for sandbox-exec that drops `-f PROFILE` and
// runs the rest. It keeps these tests about what the launcher assembles, which is the part
// that is the same on every platform; the boundary itself is asserted separately and only
// where it exists.
func passthroughSandbox(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	stub := filepath.Join(dir, "sandbox-exec")
	body := "#!/bin/sh\nshift 2\nexec \"$@\"\n"
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	old := sandboxExec
	sandboxExec = stub
	t.Cleanup(func() { sandboxExec = old })
}

// healthy is a server answering /health with the given code and /props with the context
// config/agent.env serves, which is the one the checkouts these tests build declare against.
func healthy(t *testing.T, code int) string {
	t.Helper()
	return serving(t, code, 49152)
}

// serving is the same with the served context chosen, because that is now what a session's
// budget is derived from: the file names a window, and the server is what decides it.
func serving(t *testing.T, code, nctx int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if code != http.StatusOK || r.URL.Path != "/props" {
			w.WriteHeader(code)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"model_path":"/cache/Qwen3.8-27B-Q4_K_M.gguf",
			"default_generation_settings":{"n_ctx":%d}}`, nctx)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// The property the whole feature rests on: a session leaves the repository it visited
// exactly as it found it. Asserted against the launcher, which is the part this repo owns.
func TestRunWritesNothingToTheRepository(t *testing.T) {
	root := fakeCheckout(t)
	stubClaude(t, "exit 0")
	passthroughSandbox(t)
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "only.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	if code, err := run(opts{ceiling: 100, calls: 30, sessions: 1, checkout: root, endpoint: healthy(t, http.StatusOK), noServe: true}); err != nil || code != 0 {
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
	passthroughSandbox(t)
	t.Chdir(t.TempDir())

	if code, err := run(opts{ceiling: 100, calls: 30, sessions: 1, checkout: root, endpoint: healthy(t, http.StatusOK), noServe: true}); err != nil || code != 0 {
		t.Fatalf("run: code %d, err %v", code, err)
	}
	got, err := os.ReadFile(argv)
	if err != nil {
		t.Fatal(err)
	}
	args := argsOf(got)
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
	passthroughSandbox(t)
	t.Chdir(t.TempDir())

	if _, err := run(opts{ceiling: 100, calls: 30, sessions: 1, checkout: root, endpoint: healthy(t, http.StatusOK), noServe: true}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(argv)
	// An exact token: --append-system-prompt contains "-p" and is not it.
	if slices.Contains(argsOf(got), "-p") {
		t.Fatalf("no prompt was given, so the session must not be -p: %s", got)
	}
}

func TestRunRefusesWhenTheServerIsNotReady(t *testing.T) {
	root := fakeCheckout(t)
	stubClaude(t, "exit 0")
	passthroughSandbox(t)
	t.Chdir(t.TempDir())

	// 503 is what llama-server answers while it loads: something is listening, and it
	// cannot serve yet.
	code, err := run(opts{ceiling: 100, calls: 30, sessions: 1, checkout: root, endpoint: healthy(t, http.StatusServiceUnavailable), noServe: true})
	if code != 2 || err == nil {
		t.Fatalf("a loading server must refuse, got code %d err %v", code, err)
	}
}

func TestRunPropagatesTheAgentsExitCode(t *testing.T) {
	root := fakeCheckout(t)
	stubClaude(t, "exit 3")
	passthroughSandbox(t)
	t.Chdir(t.TempDir())

	code, err := run(opts{ceiling: 100, calls: 30, sessions: 1, checkout: root, endpoint: healthy(t, http.StatusOK), noServe: true})
	if err != nil || code != 3 {
		t.Fatalf("want exit 3 passed through, got %d err %v", code, err)
	}
}

// -no-serve is what a script wants: it must refuse rather than spend twenty seconds and
// most of the machine's memory on the caller's behalf.
// A bound below one runs no session at all: the loop skips its body and the chain records
// that it stopped at its bound, one session before it began. Refused before the server,
// because finding it out by starting a model costs twenty seconds and most of the machine.
func TestABoundBelowOneIsRefusedBeforeAnythingIsStarted(t *testing.T) {
	for _, bound := range []int{0, -3} {
		code, err := run(opts{ceiling: 100, calls: 30, sessions: bound, checkout: t.TempDir()})
		if code != 2 || err == nil {
			t.Fatalf("-sessions %d: want a refusal, got code %d err %v", bound, code, err)
		}
		if !strings.Contains(err.Error(), "runs nothing") {
			t.Fatalf("-sessions %d: the refusal must say what it refuses: %v", bound, err)
		}
	}
}

func TestRunRefusesRatherThanServingWhenToldNotTo(t *testing.T) {
	root := fakeCheckout(t)
	stubClaude(t, "exit 0")
	passthroughSandbox(t)
	t.Chdir(t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening now

	code, err := run(opts{ceiling: 100, calls: 30, sessions: 1, checkout: root, endpoint: url, noServe: true})
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

// argsOf reads the stub's record, which is one argument per line — arguments here contain
// spaces, so splitting on whitespace would invent tokens that were never passed.
func argsOf(b []byte) []string {
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

// envOf reads the stub's record of its own environment, one variable per line.
func envOf(t *testing.T, argv string) []string {
	t.Helper()
	b, err := os.ReadFile(argv + ".env")
	if err != nil {
		t.Fatalf("the stub recorded no environment: %v", err)
	}
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

// handoffDirOf is where a session's budget is, which is the directory the supervisor tells
// it about through LOCALCODE_HANDOFF_DIR rather than through an argument.
func handoffDirOf(t *testing.T, argv string) string {
	t.Helper()
	for _, kv := range envOf(t, argv) {
		if v, ok := strings.CutPrefix(kv, "LOCALCODE_HANDOFF_DIR="); ok {
			return v
		}
	}
	t.Fatalf("the session was told no handoff directory")
	return ""
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

// A config the machine cannot serve must be reported when the server dies, not waited out:
// the default names a build that is not the binary on PATH, so this is the failure a machine
// without it meets first.
func TestEnsureServerReportsAServerThatDiedInsteadOfWaiting(t *testing.T) {
	root := fakeCheckout(t)
	dir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\necho 'names a server that is not there' >&2\nexit 2\n"
	if err := os.WriteFile(filepath.Join(dir, "serve.sh"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening, so the only thing that can end the wait is the exit

	done := make(chan error, 1)
	go func() { done <- ensureServer(root, url, "config/driver-mtp-32k.env", false) }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "exited before it was ready") {
			t.Fatalf("want the exit reported, got %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("ensureServer waited out a server that had already exited")
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

func TestSandboxProfileConfinesWritesAndLeavesOrdinaryReadsAlone(t *testing.T) {
	state, cwd := t.TempDir(), t.TempDir()
	path, err := writeSandboxProfile(state, cwd, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)

	if !strings.Contains(got, "(deny file-write*)") {
		t.Fatalf("writes must be denied by default:\n%s", got)
	}
	// Reads are restricted to the credential roots and nowhere else: an agent that
	// cannot read a toolchain cannot use one.
	if strings.Contains(got, "(deny file-read*)") {
		t.Fatalf("reads must stay open outside the credential roots:\n%s", got)
	}
	resolved, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, resolved) {
		t.Fatalf("the working directory must be writable:\n%s", got)
	}
	// Resolved, because /var and /tmp are symlinks into /private on macOS and seatbelt
	// matches the resolved path — an unresolved entry denies what it appears to allow.
	//
	// Asserted as the property and not as one platform's symlink. Looking for the literal
	// absence of "/tmp" said "unresolved" on a machine where /tmp resolves to itself, and
	// failed on Linux for years without anything being wrong.
	tmp := os.TempDir()
	resolvedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, sbplString(resolvedTmp)) {
		t.Fatalf("the temp directory must reach the profile resolved:\n%s", got)
	}
	if resolvedTmp != tmp && strings.Contains(got, sbplString(tmp)) {
		t.Fatalf("an unresolved path denies what it appears to allow:\n%s", got)
	}
}

// The credential roots reach the profile denied, resolved, and after the allow — the
// order is the policy, because seatbelt applies the last rule that matches.
func TestSandboxProfileDeniesReadingTheCredentialRoots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	resolvedHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	path, err := writeSandboxProfile(t.TempDir(), t.TempDir(), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)

	for _, root := range credentialRoots {
		want := sbplString(filepath.Join(resolvedHome, filepath.FromSlash(root)))
		if !strings.Contains(got, want) {
			t.Fatalf("%s must be denied, resolved:\n%s", want, got)
		}
	}
	if strings.Index(got, "(deny file-read*") < strings.Index(got, "(allow default)") {
		t.Fatalf("a deny before the allow is overridden by it:\n%s", got)
	}
}

// The boundary itself, on the machine that has one. HOME is a directory this test made, so
// the credential root probed holds a string this test wrote and nothing of anyone's.
func TestSandboxRefusesToReadACredentialRoot(t *testing.T) {
	if _, err := os.Stat("/usr/bin/sandbox-exec"); err != nil {
		t.Skip("no seatbelt on this platform")
	}
	home, cwd := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(home, ".ssh", "id_rsa")
	if err := os.WriteFile(secret, []byte("stand-in, not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(home, ".cache", "probe")
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache, []byte("cached"), 0o644); err != nil {
		t.Fatal(err)
	}
	ordinary := filepath.Join(cwd, "source.txt")
	if err := os.WriteFile(ordinary, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	profile, err := writeSandboxProfile(t.TempDir(), cwd, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	read := func(path string) (string, error) {
		out, err := exec.Command("/usr/bin/sandbox-exec", "-f", profile,
			"/bin/cat", path).CombinedOutput()
		return string(out), err
	}

	out, err := read(secret)
	if err == nil {
		t.Fatalf("the credential root was readable: %s", out)
	}
	if !strings.Contains(out, "Operation not permitted") {
		t.Fatalf("the refusal must be the sandbox's, got %q", out)
	}
	if strings.Contains(out, "stand-in") {
		t.Fatalf("the contents reached the session: %s", out)
	}
	// The repository and the caches are the reads the work is made of.
	for _, path := range []string{ordinary, cache} {
		if out, err := read(path); err != nil {
			t.Fatalf("reading %s must still work: %v: %s", path, err, out)
		}
	}
	// And a toolchain, which is the reason reads are open at all.
	if _, err := exec.LookPath("go"); err == nil {
		if out, err := exec.Command("/usr/bin/sandbox-exec", "-f", profile,
			"go", "version").CombinedOutput(); err != nil {
			t.Fatalf("a toolchain must still run: %v: %s", err, out)
		}
	}
}

func TestSandboxProfileQuotesAPathThatWouldEndTheString(t *testing.T) {
	got := sbplString(`/a/"b`)
	if got != `"/a/\"b"` {
		t.Fatalf("an unescaped quote would change the policy: %s", got)
	}
}

// The boundary itself, on the machine that has one. Everything destructive here aims at a
// directory this test created, so a profile that failed open would destroy only that.
func TestSandboxRefusesAWriteOutsideTheWorkingDirectory(t *testing.T) {
	if _, err := os.Stat("/usr/bin/sandbox-exec"); err != nil {
		t.Skip("no seatbelt on this platform")
	}
	state, cwd := t.TempDir(), t.TempDir()
	profile, err := writeSandboxProfile(state, cwd, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(home, ".localcode-test-probe-"+filepath.Base(t.TempDir()))
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(outside) })
	keep := filepath.Join(outside, "keep.txt")
	if err := os.WriteFile(keep, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Inside is allowed.
	in := exec.Command("/usr/bin/sandbox-exec", "-f", profile, "/bin/sh", "-c",
		"echo ok > "+filepath.Join(cwd, "f.txt"))
	if out, err := in.CombinedOutput(); err != nil {
		t.Fatalf("a write in the working directory must succeed: %v: %s", err, out)
	}
	// Outside is not, and the file survives to prove it.
	out := exec.Command("/usr/bin/sandbox-exec", "-f", profile, "/bin/sh", "-c", "rm -rf "+outside)
	if err := out.Run(); err == nil {
		t.Fatal("rm -rf outside the working directory was permitted")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("the file outside was destroyed: %v", err)
	}
}

// writableConfig writes the widening config for a home this test owns, and answers with
// that home resolved — which is the form a path takes by the time it is compared to a
// denied root or written into the profile.
func writableConfig(t *testing.T, body string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "localcode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "writable"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestExtraWritableReadsTheConfigAndExpandsHome(t *testing.T) {
	home := writableConfig(t, "# what this machine's ecosystems need\n\n~/.cargo\n/opt/homebrew/var\n")

	var said strings.Builder
	got := extraWritable(&said)
	want := []string{filepath.Join(home, ".cargo"), "/opt/homebrew/var"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v (comments and blanks dropped, ~ expanded)", got, want)
	}
	// A file that widens the policy in silence is the hole nobody is told about, and -net
	// announces a smaller one.
	for _, p := range want {
		if !strings.Contains(said.String(), p) {
			t.Fatalf("each path opened must be named: %q", said.String())
		}
	}
}

// The deny-list is only a boundary while the widening file cannot undo it: a session that
// may write ~/.ssh can move the key out without ever reading it.
func TestExtraWritableRefusesALineThatOpensACredentialRoot(t *testing.T) {
	home := writableConfig(t, "~/.ssh\n~/\n/\n~/.cargo\n")

	var said strings.Builder
	got := extraWritable(&said)
	if len(got) != 1 || got[0] != filepath.Join(home, ".cargo") {
		t.Fatalf("only the line reaching no credential root may be applied: %v", got)
	}
	if strings.Count(said.String(), "refused") != 3 {
		t.Fatalf("the root itself, its parent and / must each be refused:\n%s", said.String())
	}
}

// The endpoint has to stay reachable or nothing works, and everything else has to be
// unreachable or the sandbox is not a boundary for the repository's source. Both halves
// are asserted against a real listener rather than by reading the profile.
func TestSandboxAllowsLoopbackAndRefusesTheInternet(t *testing.T) {
	if _, err := os.Stat("/usr/bin/sandbox-exec"); err != nil {
		t.Skip("no seatbelt on this platform")
	}
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("no curl to probe with")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("reached"))
	}))
	defer srv.Close()

	state, cwd := t.TempDir(), t.TempDir()
	profile, err := writeSandboxProfile(state, cwd, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	probe := func(url string) string {
		out, _ := exec.Command("/usr/bin/sandbox-exec", "-f", profile,
			"/usr/bin/curl", "-s", "-m", "5", url).CombinedOutput()
		return string(out)
	}

	// httptest listens on loopback, which is where the model is too.
	if got := probe(srv.URL); !strings.Contains(got, "reached") {
		t.Fatalf("loopback must stay reachable, got %q", got)
	}
	// 192.0.2.0/24 is TEST-NET-1: reserved, routable-looking, and never a real host, so a
	// refusal here is the sandbox rather than someone's firewall.
	if got := probe("http://192.0.2.1/"); strings.Contains(got, "reached") {
		t.Fatalf("outbound must be refused, got %q", got)
	}
}

func TestNetOpensOutboundForTheSession(t *testing.T) {
	state, cwd := t.TempDir(), t.TempDir()
	path, err := writeSandboxProfile(state, cwd, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "(deny network*)") {
		t.Fatalf("-net must lift the network denial:\n%s", body)
	}
	// Writes stay confined either way: -net is about reachability, not about the filesystem.
	if !strings.Contains(string(body), "(deny file-write*)") {
		t.Fatalf("-net must not widen writes:\n%s", body)
	}
}

// The session is budgeted before it starts, and it is told where to write in the same
// breath: the directory it may write to and the directory the spec names are one, or the
// only call the gate permits is the one the harness refuses.
func TestRunBudgetsTheSessionAndOpensTheDirectoryItMustWrite(t *testing.T) {
	root := fakeCheckout(t)
	argv := stubClaude(t, "exit 0")
	passthroughSandbox(t)
	t.Chdir(t.TempDir())

	if code, err := run(opts{ceiling: 100, calls: 30, sessions: 1, checkout: root,
		endpoint: healthy(t, http.StatusOK), noServe: true}); err != nil || code != 0 {
		t.Fatalf("run: code %d err %v", code, err)
	}
	got, err := os.ReadFile(argv)
	if err != nil {
		t.Fatal(err)
	}
	dir := flagValue(argsOf(got), "--add-dir")
	if dir == "" {
		t.Fatalf("the session was given no directory to write its handoff in:\n%s", got)
	}
	// The budget is in the session's own directory and the handoff path is the chain's:
	// the prompt names the second so it does not move between sessions, and the hook
	// finds the first through the environment rather than through an argument.
	spec, err := chain.ReadSpec(handoffDirOf(t, argv))
	if err != nil {
		t.Fatalf("the session was not budgeted: %v", err)
	}
	if spec.Handoff != filepath.Join(dir, chain.HandoffName) {
		t.Fatalf("the handoff must be in the directory the session may write: %s", spec.Handoff)
	}
	// The harness's own check on the declaration is off, or a quarter of the window is held
	// back by an undocumented fraction this repo cannot account for.
	const off = "CLAUDE_CODE_DISABLE_UNKNOWN_MODEL_WINDOW_ENFORCEMENT=1"
	if env := envOf(t, argv); !slices.Contains(env, off) {
		t.Fatalf("the session must run with %s", off)
	}
	// The headroom of a 45,056 window: less a quarter for a turn's results, less the 6,144
	// the turns after the gate's last reading were measured to generate.
	if spec.Limits.Ceiling != 27648 || spec.Limits.Calls != 30 {
		t.Fatalf("the flags must reach the session: %+v", spec.Limits)
	}
}

// A window nothing fits in is refused before a cold ingest is spent discovering it. The
// server is what starves it, because the server is what the window is now taken from: 8,192
// served less the 4,096 kept for a reply leaves a session less than its own preamble.
func TestRunRefusesAWindowNothingFitsIn(t *testing.T) {
	root := fakeCheckout(t)
	stubClaude(t, "exit 0")
	passthroughSandbox(t)
	t.Chdir(t.TempDir())

	code, err := run(opts{ceiling: 100, calls: 30, sessions: 1, checkout: root,
		endpoint: serving(t, http.StatusOK, 8192), noServe: true})
	if code != 2 || err == nil {
		t.Fatalf("a window under the preamble must be refused: code %d err %v", code, err)
	}
}

// The point of the change: naming a config moves the wall and the budget together. The
// file declares 45,056 throughout these tests; a server at 32,768 must budget the session
// for that instead, or a chain runs against a wall 16,384 tokens further away than the one
// it has and dies on `Prompt is too long` with no handoff written.
func TestRunBudgetsAgainstTheServedContextAndNotTheFile(t *testing.T) {
	root := fakeCheckout(t)
	argv := stubClaude(t, "exit 0")
	passthroughSandbox(t)
	t.Chdir(t.TempDir())

	if code, err := run(opts{ceiling: 100, calls: 30, sessions: 1, checkout: root,
		endpoint: serving(t, http.StatusOK, 32768), noServe: true}); err != nil || code != 0 {
		t.Fatalf("run: code %d err %v", code, err)
	}
	spec, err := chain.ReadSpec(handoffDirOf(t, argv))
	if err != nil {
		t.Fatalf("the session was not budgeted: %v", err)
	}
	// 32,768 served is declared whole; less the 4,096 the harness keeps whatever it is
	// told, and three quarters of the rest is the 21,504 it will actually send; less a
	// quarter of that for a turn's results and 6,144 for the turns is the ceiling.
	if spec.Limits.Window != 28672 || spec.Limits.Ceiling != 15360 {
		t.Fatalf("the served context must be what bounds the session: %+v", spec.Limits)
	}
}

// A server that will not say what it serves cannot be budgeted against, and guessing is
// the failure this whole change is about.
func TestRunRefusesAServerThatReportsNoContext(t *testing.T) {
	root := fakeCheckout(t)
	stubClaude(t, "exit 0")
	passthroughSandbox(t)
	t.Chdir(t.TempDir())

	code, err := run(opts{ceiling: 100, calls: 30, sessions: 1, checkout: root,
		endpoint: serving(t, http.StatusOK, 0), noServe: true})
	if code != 2 || err == nil {
		t.Fatalf("a server reporting no context must be refused: code %d err %v", code, err)
	}
}

// The gate end to end: the harness's payload in, the harness's exit code out.
func TestHookRefusesWithExitTwoAndPermitsWithZero(t *testing.T) {
	dir := t.TempDir()
	limits, err := chain.NewLimits(45056, 4096, 50, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.WriteSpec(dir, chain.Spec{Limits: limits,
		Handoff: filepath.Join(dir, chain.HandoffName)}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCALCODE_HANDOFF_DIR", dir)

	call := `{"session_id":"s1","tool_name":"Bash","tool_input":{"command":"go test ./..."}}`
	for i := 1; i <= limits.Calls; i++ {
		if code, err := hookWith(call); err != nil || code != 0 {
			t.Fatalf("call %d must be permitted: code %d err %v", i, code, err)
		}
	}
	if code, err := hookWith(call); err != nil || code != 2 {
		t.Fatalf("the call past the budget must be refused with 2: code %d err %v", code, err)
	}
	// And the way out stays open, or the session dies holding what it learned.
	out := `{"session_id":"s1","tool_name":"Write","tool_input":{"file_path":"` + dir + `/HANDOFF.md"}}`
	if code, err := hookWith(out); err != nil || code != 0 {
		t.Fatalf("the handoff must still be permitted: code %d err %v", code, err)
	}
}

// An unbudgeted session is left alone rather than refused: standing aside leaves it
// behaving as it did before this feature, and refusing would make it useless in silence.
func TestHookStandsAsideWhenNothingBudgetedTheSession(t *testing.T) {
	t.Setenv("LOCALCODE_HANDOFF_DIR", t.TempDir())
	code, err := hookWith(`{"session_id":"s1","tool_name":"Bash","tool_input":{}}`)
	if err != nil || code != 0 {
		t.Fatalf("code %d err %v", code, err)
	}
}

// hookWith runs the hook against a payload, with stdin standing in for the harness.
func hookWith(payload string) (int, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return 0, err
	}
	if _, err := w.WriteString(payload); err != nil {
		return 0, err
	}
	_ = w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old; _ = r.Close() }()
	return hook("gate")
}

// The clock bounds a session nobody is watching. On one somebody is, it would end the work
// mid-thought: an interactive session is cancelled with Ctrl-C, not by a timer.
func TestTheClockDoesNotRunOnAnInteractiveSession(t *testing.T) {
	root := fakeCheckout(t)
	// Longer than the timeout, and it must still be allowed to finish.
	stubClaude(t, "sleep 1")
	passthroughSandbox(t)
	t.Chdir(t.TempDir())

	quiet(t)
	code, err := run(opts{ceiling: 100, calls: 30, sessions: 4, timeout: 50 * time.Millisecond,
		checkout: root, endpoint: healthy(t, http.StatusOK), noServe: true})
	if err != nil || code != 0 {
		t.Fatalf("an interactive session must not be stopped by the clock: code %d err %v", code, err)
	}
}

// `kit claim` prints `git worktree add ../<repo>-<id>`, and that failed under the sandbox
// with `Operation not permitted` on a real repository: writes stopped at the working
// directory. The model recovered by putting the worktree inside the repository, which is
// where nobody looks for it. Asserted against the kernel, because the profile being
// readable is not the same as the boundary being where it says.
func TestSandboxAllowsAWorktreeBesideTheRepositoryAndNothingElseBesideIt(t *testing.T) {
	if _, err := os.Stat("/usr/bin/sandbox-exec"); err != nil {
		t.Skip("no seatbelt on this platform")
	}
	// Under the home directory, not under a temp one: the temp directory is writable
	// anyway, so a refusal asserted there would prove nothing. A parent with a dot in its
	// name, as a real one has: gitlab.ubnk.uz.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(home, ".localcode-test-"+filepath.Base(t.TempDir()), "gitlab.example.uz")
	cwd := filepath.Join(parent, "slack-stats")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(parent)) })
	profile, err := writeSandboxProfile(t.TempDir(), cwd, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	mkdir := func(path string) error {
		return exec.Command("/usr/bin/sandbox-exec", "-f", profile, "/bin/mkdir", path).Run()
	}

	// The worktree kit asks for.
	if err := mkdir(filepath.Join(parent, "slack-stats-0001")); err != nil {
		t.Fatalf("a worktree beside the repository must be allowed: %v", err)
	}
	// Every other project of this developer's lives beside it too, and stays out of reach.
	if err := mkdir(filepath.Join(parent, "somebody-elses-repo")); err == nil {
		t.Fatal("the parent directory must not be opened up, only the repository's own siblings")
	}
	// And a name that merely looks like one: `slack-statsX-1` must not match `slack-stats`.
	if err := mkdir(filepath.Join(parent, "slack-statsX-1")); err == nil {
		t.Fatal("the separator is part of the name, not a wildcard")
	}
}

// A repository whose name carries regex punctuation must not widen the pattern.
func TestSiblingWorktreePatternEscapesTheRepositorysName(t *testing.T) {
	dir := t.TempDir()
	cwd := filepath.Join(dir, "a.b+c")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	got := siblingWorktrees(cwd)
	if strings.Contains(got, "a.b+c") {
		t.Fatalf("the name must reach the pattern escaped: %s", got)
	}
	if !strings.HasSuffix(got, `a\.b\+c-[^/]+`) {
		t.Fatalf("got %s", got)
	}
}

// A session cannot find out where a worktree may go except by being refused, and the one
// that was refused put it somewhere nobody looks for it. The briefing is the trusted
// channel, so it is where the two permitted places are named.
func TestBriefingNamesWhereAWorktreeMayGo(t *testing.T) {
	plain := t.TempDir()
	got := sandboxBriefing(plain)
	if !strings.Contains(got, "../"+filepath.Base(plain)+"-<name>") {
		t.Fatalf("the sibling the sandbox allows must be named: %s", got)
	}
	if strings.Contains(got, ".worktrees") {
		t.Fatalf("a repository that keeps no .worktrees must not be told to use one: %s", got)
	}

	// A repository that already keeps one has decided where they go.
	kept := t.TempDir()
	if err := os.Mkdir(filepath.Join(kept, ".worktrees"), 0o755); err != nil {
		t.Fatal(err)
	}
	got = sandboxBriefing(kept)
	if !strings.Contains(got, "`.worktrees/<name>`") {
		t.Fatalf(".worktrees must be named first where it exists: %s", got)
	}
	if strings.Index(got, ".worktrees") > strings.Index(got, "../"+filepath.Base(kept)) {
		t.Fatalf("the repository's own choice comes first: %s", got)
	}
}

// A refusal a session cannot explain is a refusal it works around, so the briefing that
// already names what may be written names what may not be read, and why.
func TestBriefingNamesTheDeniedRootsAndWhy(t *testing.T) {
	got := sandboxBriefing(t.TempDir())
	for _, root := range credentialRoots {
		if !strings.Contains(got, "~/"+root) {
			t.Fatalf("a denied root the session is not told about is a puzzle: %s", got)
		}
	}
	if !strings.Contains(got, "credentials") {
		t.Fatalf("the reason is what stops the session working around it: %s", got)
	}
}

// piCheckout adds what the pi adapter reads out of a checkout: the provider file it loads
// and takes the model id and the output reservation from, the extension that holds a
// session to its budget, and the settings a session is served.
func piCheckout(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "harness", "pi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	provider := "export default async function (pi) {\n  pi.registerProvider(\"local\", {\n" +
		"    models: [\n      {\n        id: \"served/model:Q4\",\n        maxTokens: 4096,\n" +
		"      },\n    ],\n  });\n}\n"
	for name, body := range map[string]string{
		"local-provider.js":       provider,
		"localcode-gate.js":       "export default function (pi) {}\n",
		"settings.json.reference": "{}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// The point of the flag: one token changed runs the same instruction in another agent,
// with the same tool set, the same briefing and the same budget.
func TestHarnessPiStartsTheSessionPiWouldRun(t *testing.T) {
	root := fakeCheckout(t)
	piCheckout(t, root)
	argv := stubAgent(t, "pi", "exit 0")
	passthroughSandbox(t)
	quiet(t)
	t.Chdir(t.TempDir())

	if code, err := run(opts{harness: "pi", ceiling: 100, calls: 30, sessions: 1, checkout: root,
		endpoint: healthy(t, http.StatusOK), noServe: true, args: []string{"fix", "the", "tests"}}); err != nil {
		t.Fatalf("run: code %d err %v", code, err)
	}
	got, err := os.ReadFile(argv)
	if err != nil {
		t.Fatal(err)
	}
	args := argsOf(got)
	for _, want := range []string{"--tools", "read,edit,write,bash", "--provider", "local",
		"--model", "served/model:Q4", "-p", "fix the tests"} {
		if !slices.Contains(args, want) {
			t.Errorf("the session was not given %q:\n%s", want, got)
		}
	}
	// The briefing the incumbent's arm gets, or the two arms differ by more than the flag.
	// Searched over the whole record rather than one argument: the briefing has newlines
	// in it, and the stub writes an argument per line.
	for _, want := range []string{"HANDOFF.md", "sandbox", "tool calls"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("the briefing does not mention %q:\n%s", want, got)
		}
	}
	// The session's own directory is where pi is told to write, so what it cost is read
	// back without searching for it.
	dir := handoffDirOf(t, argv)
	if flagValue(args, "--session-dir") != dir {
		t.Errorf("pi must write its session where the driver kept it: %q", flagValue(args, "--session-dir"))
	}
	spec, err := chain.ReadSpec(dir)
	if err != nil {
		t.Fatalf("the session was not budgeted: %v", err)
	}
	if spec.Harness != "pi" {
		t.Errorf("the session's record must say which agent ran it: %q", spec.Harness)
	}
	// 49,152 served whole, less the 4,096 the provider file declares for a reply.
	if spec.Limits.Window != 45056 {
		t.Errorf("the window must be the served context less pi's own reservation: %+v", spec.Limits)
	}
}

// A harness nobody drives is refused by name, with the ones that are: a chain silently run
// in another agent is a comparison of nothing.
func TestAnUnknownHarnessIsRefusedAndNamesTheOnesThereAre(t *testing.T) {
	root := fakeCheckout(t)
	t.Chdir(t.TempDir())
	code, err := run(opts{harness: "cursor", ceiling: 100, sessions: 1, checkout: root,
		endpoint: healthy(t, http.StatusOK), noServe: true})
	if code != 2 || err == nil {
		t.Fatalf("an unknown harness must be refused: code %d err %v", code, err)
	}
	for _, want := range []string{"cursor", "claude-code", "pi"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
}
