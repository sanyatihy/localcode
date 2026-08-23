package main

import (
	"encoding/json"
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
	passthroughSandbox(t)
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "only.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	if code, err := run(opts{ceiling: 100, calls: 30, checkout: root, endpoint: healthy(t, http.StatusOK), noServe: true}); err != nil || code != 0 {
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

	if code, err := run(opts{ceiling: 100, calls: 30, checkout: root, endpoint: healthy(t, http.StatusOK), noServe: true}); err != nil || code != 0 {
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

	if _, err := run(opts{ceiling: 100, calls: 30, checkout: root, endpoint: healthy(t, http.StatusOK), noServe: true}); err != nil {
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
	code, err := run(opts{ceiling: 100, calls: 30, checkout: root, endpoint: healthy(t, http.StatusServiceUnavailable), noServe: true})
	if code != 2 || err == nil {
		t.Fatalf("a loading server must refuse, got code %d err %v", code, err)
	}
}

func TestRunPropagatesTheAgentsExitCode(t *testing.T) {
	root := fakeCheckout(t)
	stubClaude(t, "exit 3")
	passthroughSandbox(t)
	t.Chdir(t.TempDir())

	code, err := run(opts{ceiling: 100, calls: 30, checkout: root, endpoint: healthy(t, http.StatusOK), noServe: true})
	if err != nil || code != 3 {
		t.Fatalf("want exit 3 passed through, got %d err %v", code, err)
	}
}

// -no-serve is what a script wants: it must refuse rather than spend twenty seconds and
// most of the machine's memory on the caller's behalf.
func TestRunRefusesRatherThanServingWhenToldNotTo(t *testing.T) {
	root := fakeCheckout(t)
	stubClaude(t, "exit 0")
	passthroughSandbox(t)
	t.Chdir(t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening now

	code, err := run(opts{ceiling: 100, calls: 30, checkout: root, endpoint: url, noServe: true})
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

func TestSandboxProfileConfinesWritesAndLeavesReadsAlone(t *testing.T) {
	state, cwd := t.TempDir(), t.TempDir()
	path, err := writeSandboxProfile(state, cwd, false)
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
	// Reads are deliberately not restricted: an agent that cannot read a toolchain
	// cannot use one.
	if strings.Contains(got, "(deny file-read") {
		t.Fatalf("reads must stay open:\n%s", got)
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
	profile, err := writeSandboxProfile(state, cwd, false)
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

func TestExtraWritableReadsTheConfigAndExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "localcode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# what this machine's ecosystems need\n\n~/.cargo\n/opt/homebrew/var\n"
	if err := os.WriteFile(filepath.Join(dir, "writable"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	got := extraWritable()
	want := []string{filepath.Join(home, ".cargo"), "/opt/homebrew/var"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v (comments and blanks dropped, ~ expanded)", got, want)
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
	profile, err := writeSandboxProfile(state, cwd, false)
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
	path, err := writeSandboxProfile(state, cwd, true)
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

// The gate is this binary, not a fourth shell script, so what enforces the budget is
// covered by the same `make check` as everything else that decides something.
func TestSettingsInstallTheGateAsThisBinary(t *testing.T) {
	root, dir := fakeCheckout(t), t.TempDir()
	path, err := writeSettings(root, dir, true)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Hooks map[string][]struct {
			Hooks []struct{ Command string } `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	pre, ok := doc.Hooks["PreToolUse"]
	if !ok || len(pre) == 0 || len(pre[0].Hooks) == 0 {
		t.Fatalf("no PreToolUse hook is installed:\n%s", body)
	}
	if !strings.HasSuffix(pre[0].Hooks[0].Command, " hook gate") {
		t.Fatalf("the gate must be this binary run as a hook: %q", pre[0].Hooks[0].Command)
	}
}

// Stop fires whenever the agent finishes responding, which in an interactive session is
// every time it hands the keyboard back. Installed there it would refuse the conversation.
func TestStopIsInstalledOnlyForASessionAnsweringOneInstruction(t *testing.T) {
	root := fakeCheckout(t)
	installed := func(oneShot bool) bool {
		path, err := writeSettings(root, t.TempDir(), oneShot)
		if err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Hooks map[string]any `json:"hooks"`
		}
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatal(err)
		}
		_, ok := doc.Hooks["Stop"]
		return ok
	}
	if !installed(true) {
		t.Fatal("a session answering one instruction must not be able to end without a handoff")
	}
	if installed(false) {
		t.Fatal("an interactive session would be refused at the end of every turn")
	}
}

// An installation under a path with a space in it would otherwise run its first word.
func TestShellQuoteSurvivesAPathAShellWouldSplit(t *testing.T) {
	if got := shellQuote("/Users/a b/bin/localcode"); got != "'/Users/a b/bin/localcode'" {
		t.Fatalf("got %s", got)
	}
	if got := shellQuote("/it's/here"); got != `'/it'\''s/here'` {
		t.Fatalf("a quote in the path must not end the quoting: %s", got)
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

	if code, err := run(opts{ceiling: 100, calls: 30, checkout: root,
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
	spec, err := chain.ReadSpec(dir)
	if err != nil {
		t.Fatalf("the session was not budgeted: %v", err)
	}
	if spec.Handoff != filepath.Join(dir, chain.HandoffName) {
		t.Fatalf("the handoff must be in the directory the session may write: %s", spec.Handoff)
	}
	// The headroom of a 40,960 window: less a quarter for a turn's results, less twice the
	// 4,096 output reservation.
	if spec.Limits.Ceiling != 22528 || spec.Limits.Calls != 30 {
		t.Fatalf("the flags must reach the session: %+v", spec.Limits)
	}
}

// A window nothing fits in is refused before a cold ingest is spent discovering it.
func TestRunRefusesAWindowNothingFitsIn(t *testing.T) {
	root := fakeCheckout(t)
	stubClaude(t, "exit 0")
	passthroughSandbox(t)
	t.Chdir(t.TempDir())
	env := `ANTHROPIC_BASE_URL="http://127.0.0.1:8081"` + "\n" +
		`CLAUDE_CODE_MAX_CONTEXT_TOKENS="8192"` + "\n" +
		`CLAUDE_CODE_MAX_OUTPUT_TOKENS="4096"` + "\n"
	if err := os.WriteFile(filepath.Join(root, "harness", "claude-code", "claude-code.env"),
		[]byte(env), 0o644); err != nil {
		t.Fatal(err)
	}
	code, err := run(opts{ceiling: 100, calls: 30, checkout: root,
		endpoint: healthy(t, http.StatusOK), noServe: true})
	if code != 2 || err == nil {
		t.Fatalf("a window under the preamble must be refused: code %d err %v", code, err)
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
	profile, err := writeSandboxProfile(t.TempDir(), cwd, false)
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
	if !strings.Contains(got, "../"+filepath.Base(plain)+"-<id>") {
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
	if !strings.Contains(got, "`.worktrees/<id>`") {
		t.Fatalf(".worktrees must be named first where it exists: %s", got)
	}
	if strings.Index(got, ".worktrees") > strings.Index(got, "../"+filepath.Base(kept)) {
		t.Fatalf("the repository's own choice comes first: %s", got)
	}
}
