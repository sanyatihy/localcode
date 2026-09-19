package scripts

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// stubPATH writes a `sysctl` reporting totalBytes of memory and a GPU cap of capMB, and a
// `sudo` that records its arguments instead of running them. Both are shelled out to by
// name, so a directory first on PATH is the whole seam. It returns the PATH to run with and
// the file `sudo` would have written to, which stays absent when it was never called.
func stubPATH(t *testing.T, totalBytes, capMB string) (path, sudoLog string) {
	t.Helper()
	dir := t.TempDir()
	sudoLog = filepath.Join(dir, "sudo.log")
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("sysctl", "#!/bin/sh\ncase \"$*\" in\n"+
		"*hw.memsize*) echo "+totalBytes+" ;;\n"+
		"*iogpu.wired_limit_mb*) echo "+capMB+" ;;\n"+
		"*) exit 1 ;;\nesac\n")
	write("sudo", "#!/bin/sh\necho \"$@\" >> \""+sudoLog+"\"\n")
	return dir + string(os.PathListSeparator) + os.Getenv("PATH"), sudoLog
}

// gpuraise runs the script with those stubs and returns its exit code and its output.
func gpuraise(t *testing.T, totalBytes, capMB string, args ...string) (int, string, string) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("iogpu.wired_limit_mb is a macOS sysctl, and the script refuses elsewhere")
	}
	script, err := filepath.Abs("gpuraise.sh")
	if err != nil {
		t.Fatal(err)
	}
	path, sudoLog := stubPATH(t, totalBytes, capMB)
	cmd := exec.Command(script, args...)
	cmd.Env = append(os.Environ(), "PATH="+path)
	out, err := cmd.CombinedOutput()
	if err != nil && cmd.ProcessState == nil {
		t.Fatalf("running %s: %v", script, err)
	}
	return cmd.ProcessState.ExitCode(), string(out), sudoLog
}

func TestGpuraiseRepeatedRaiseIsANoOp(t *testing.T) {
	// 32 GiB installed, 16,384 MiB already capped, and the same value asked for again: the
	// state the node is in whenever scripts/node/cap.sh runs a second time — a reinstall, or
	// a bootstrap after the boot-time cap has already been applied.
	code, out, sudoLog := gpuraise(t, "34359738368", "16384", "16384")
	if code != 0 {
		t.Errorf("exit %d, want 0: %s", code, out)
	}
	if !strings.Contains(out, "already set") {
		t.Errorf("output does not say the cap was already there: %s", out)
	}
	if _, err := os.Stat(sudoLog); !os.IsNotExist(err) {
		log, _ := os.ReadFile(sudoLog)
		t.Errorf("a no-op shelled out to sudo: %s", log)
	}
}

func TestGpuraiseDifferentValueOverASetCapIsRefused(t *testing.T) {
	// The refusal the no-op must not have widened: with a cap set by hand the kernel's own
	// derivation is no longer readable, so a raise to another value cannot be checked.
	code, out, sudoLog := gpuraise(t, "34359738368", "16384", "20480")
	if code != 2 {
		t.Errorf("exit %d, want 2: %s", code, out)
	}
	if _, err := os.Stat(sudoLog); !os.IsNotExist(err) {
		t.Error("a refused raise shelled out to sudo")
	}
}

// serveConfig writes a config with the fields serve.sh requires, plus whatever a case adds.
func serveConfig(t *testing.T, dir, extra string) string {
	t.Helper()
	path := filepath.Join(dir, "stub.env")
	base := `MODEL_HF="stub/model:Q4_K_M"
HOST="127.0.0.1"
PORT="8081"
CTX_SIZE="32768"
CACHE_TYPE_K="q8_0"
CACHE_TYPE_V="q8_0"
FLASH_ATTN="auto"
N_GPU_LAYERS="999"
PARALLEL="1"
`
	if err := os.WriteFile(path, []byte(base+extra), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// serve runs serve.sh against a stub server that prints the arguments it was execed with and
// loads nothing. The banner is on stderr and the arguments on stdout, so both are returned
// together: what was bound is a claim the banner makes and the arguments have to agree with.
func serve(t *testing.T, extra string, env ...string) string {
	t.Helper()
	dir := t.TempDir()
	stub := filepath.Join(dir, "stub-server")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("serve.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(script, serveConfig(t, dir, extra))
	cmd.Dir = ".." // serve.sh resolves a config and a template against the caller's directory
	cmd.Env = append(append(os.Environ(), "SERVER_BIN="+stub), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("serve.sh: %v: %s", err, out)
	}
	return string(out)
}

func TestServeOptionalFlagsFollowTheConfig(t *testing.T) {
	for _, c := range []struct {
		name, extra, flag string
		want              bool
	}{
		{"projector loaded by default", "", "--no-mmproj", false},
		{"projector dropped when the config says so", "NO_MMPROJ=\"1\"\n", "--no-mmproj", true},
		{"weights unpinned by default", "", "--mlock", false},
		{"weights pinned when the config says so", "MLOCK=\"1\"\n", "--mlock", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := serve(t, c.extra)
			if got := strings.Contains(out, c.flag); got != c.want {
				t.Errorf("%s present = %v, want %v: %s", c.flag, got, c.want, out)
			}
		})
	}
}

func TestServeHostFromTheEnvironmentWinsOverTheConfig(t *testing.T) {
	// 0053's override, which the node's serving daemon is the first thing to use: the
	// committed config keeps loopback and the environment is where a machine says it serves
	// the network. The banner is read as well as the arguments, because a run's only record
	// of what was bound is that line.
	out := serve(t, "", "HOST=0.0.0.0")
	if !strings.Contains(out, "--host 0.0.0.0") {
		t.Errorf("the server was not given the environment's host: %s", out)
	}
	if strings.Contains(out, "--host 127.0.0.1") {
		t.Errorf("the config's host reached the server anyway: %s", out)
	}
	if !strings.Contains(out, "on 0.0.0.0:8081") {
		t.Errorf("the banner does not say what was bound: %s", out)
	}
}

// propsEndpoint serves one /props body and nothing else, which is all the slot readings ask
// of a server.
func propsEndpoint(t *testing.T, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/props") {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// lib calls one function from scripts/lib.sh against that endpoint and returns what it
// printed. Sourced rather than executed, which is how every script here uses it.
func lib(t *testing.T, endpoint, call string) string {
	t.Helper()
	cmd := exec.Command("bash", "-c", ". ./lib.sh; "+call)
	cmd.Env = append(os.Environ(), "ENDPOINT="+endpoint)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s: %v", call, err)
	}
	return strings.TrimSpace(string(out))
}

// llama-server reports n_ctx as one slot's share on one build and as the whole server's on
// another, and both sit beside the same total_slots. A ladder that reads the wrong one fills
// a quarter of the context it believes it is measuring, or refuses a server that is serving
// exactly what it was asked for — so the request breaks the tie, and a reply that is neither
// shape is refused rather than guessed at.
func TestPerSlotContextResolvesBothPropsShapes(t *testing.T) {
	for _, c := range []struct{ name, props, want string }{
		{"a per-slot n_ctx beside four slots",
			`{"default_generation_settings":{"n_ctx":12288},"total_slots":4}`, "12288"},
		{"a total n_ctx beside four slots",
			`{"default_generation_settings":{"n_ctx":49152},"total_slots":4}`, "12288"},
		{"one slot, where the two shapes are the same number",
			`{"default_generation_settings":{"n_ctx":49152},"total_slots":1}`, "49152"},
		{"a server that does not say how many slots it has",
			`{"default_generation_settings":{"n_ctx":49152}}`, "49152"},
		{"a served context that is neither shape",
			`{"default_generation_settings":{"n_ctx":32768},"total_slots":4}`, "ambiguous"},
		{"one slot serving something else, which the caller compares for itself",
			`{"default_generation_settings":{"n_ctx":8192},"total_slots":1}`, "8192"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := lib(t, propsEndpoint(t, c.props), "per_slot_ctx 49152"); got != c.want {
				t.Errorf("per_slot_ctx 49152 = %s, want %s", got, c.want)
			}
		})
	}

	// The slot count is read off the same response, and a backend that does not report one
	// is not a backend serving none.
	four := propsEndpoint(t, `{"default_generation_settings":{"n_ctx":12288},"total_slots":4}`)
	if got := lib(t, four, "served_slots"); got != "4" {
		t.Errorf("served_slots = %s, want 4", got)
	}
	silent := propsEndpoint(t, `{"default_generation_settings":{"n_ctx":49152}}`)
	if got := lib(t, silent, "served_slots"); got != "null" {
		t.Errorf("served_slots = %s, want null", got)
	}
	// An endpoint nothing answers on is not a server serving zero context.
	if got := lib(t, "http://127.0.0.1:1", "per_slot_ctx 49152"); got != "null" {
		t.Errorf("an unanswered endpoint resolved to %s, want null", got)
	}
}

// ladderStub is the server a ladder cell talks to: health, /props reporting two slots, and a
// completions endpoint that holds each request until both have arrived. Holding them is the
// assertion — a ladder that filled one slot at a time could never get past it. The second
// request to arrive is answered with an empty HTTP 500, which is the failure a check for the
// word "error" cannot see.
type ladderStub struct {
	mu      sync.Mutex
	prompts []string
	both    chan struct{}
	arrived int
}

func (s *ladderStub) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/health"):
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/props"):
			_, _ = io.WriteString(w, `{"model_path":"/m/Qwen.gguf","total_slots":2,`+
				`"default_generation_settings":{"n_ctx":2048}}`)
		default:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Messages []struct{ Content string } `json:"messages"`
			}
			if err := json.Unmarshal(body, &req); err != nil || len(req.Messages) == 0 {
				t.Errorf("a fill request that is not one: %v", err)
				return
			}
			s.mu.Lock()
			s.prompts = append(s.prompts, req.Messages[0].Content)
			s.arrived++
			mine := s.arrived
			if s.arrived == 2 {
				close(s.both)
			}
			s.mu.Unlock()

			select {
			case <-s.both:
			case <-time.After(30 * time.Second):
				t.Error("only one request was ever in flight: the slots were not filled at once")
			}
			if mine == 2 {
				// No body at all, which is what a server out of memory answers with.
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"OK"}}],`+
				`"timings":{"prompt_per_second":42.5}}`)
		}
	}
}

// One ladder cell against that stub. What is under test is the fill loop and the pass rule:
// a server reporting two slots is filled at both at once, each from its own prompt, and one
// slot that did not fill is the whole rung's answer.
func TestLadderFillsEverySlotAtOnceAndOneFailureFailsTheRung(t *testing.T) {
	stub := &ladderStub{both: make(chan struct{})}
	srv := httptest.NewServer(stub.handler(t))
	defer srv.Close()

	dir := t.TempDir()
	// A server process that stays up and is not llama-server: stop_server and the liveness
	// check go through PROC, so nothing on the machine running this can be killed by it.
	proc := "ladder-stub-server"
	stubBin := filepath.Join(dir, proc)
	// No `exec`: the shell has to stay in the process table under its own name, which is
	// what PROC matches and what the liveness check asks about.
	if err := os.WriteFile(stubBin, []byte("#!/bin/sh\nsleep 60\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = exec.Command("pkill", "-f", proc).Run() })

	out := filepath.Join(dir, "ceiling.jsonl")
	script, err := filepath.Abs("ladder.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(script)
	cmd.Env = append(os.Environ(),
		"CELLS=4096:q8_0", "BASE="+serveConfig(t, dir, ""), "OUT="+out,
		"ENDPOINT="+srv.URL, "SERVER_BIN="+stubBin, "PROC="+proc,
		// The node's serving daemon is not this machine's business: naming one that is
		// loaded nowhere keeps stop_server off launchctl.
		"SERVE_DAEMON=com.localcode.absent", "CONDITION=unattended", "HEALTH_GRACE=60")
	if got, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ladder.sh: %v\n%s", err, got)
	}

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.prompts) != 2 {
		t.Fatalf("%d slots were filled, want 2", len(stub.prompts))
	}
	if stub.prompts[0] == stub.prompts[1] {
		t.Error("both slots were sent the same prompt: one could be served from what the other ingested")
	}
	// 2,048 per slot at the default fill fraction. A ladder sizing the fill against the
	// config's 4,096 would send twice this.
	for i, p := range stub.prompts {
		if words := len(strings.Fields(p)); words < 1800 || words > 1900 {
			t.Errorf("slot %d was sent %d words, want ~1,843 — one slot's share of 4,096", i+1, words)
		}
	}

	rows, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var row struct {
		Outcome      string   `json:"outcome"`
		Slots        int      `json:"slots"`
		PerSlotCtx   int      `json:"per_slot_ctx"`
		SlotOutcomes []string `json:"slot_outcomes"`
	}
	cell := ""
	for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n") {
		if strings.Contains(line, `"cell"`) {
			cell = line
		}
	}
	if cell == "" {
		t.Fatalf("no cell row was written:\n%s", rows)
	}
	if err := json.Unmarshal([]byte(cell), &row); err != nil {
		t.Fatalf("cell row: %v\n%s", err, cell)
	}
	if row.Slots != 2 || row.PerSlotCtx != 2048 {
		t.Errorf("row says %d slots at %d per slot, want 2 at 2048", row.Slots, row.PerSlotCtx)
	}
	if row.Outcome != "http_500" {
		t.Errorf("outcome is %q; one slot that did not fill is the whole rung's answer", row.Outcome)
	}
	// Which slot the stub failed is a race it does not control, so the row is read for one
	// of each rather than for an order.
	ok, failed := 0, 0
	for _, o := range row.SlotOutcomes {
		switch o {
		case "ok":
			ok++
		case "http_500":
			failed++
		}
	}
	if ok != 1 || failed != 1 {
		t.Errorf("slot outcomes %v do not say which slot filled and which did not", row.SlotOutcomes)
	}
}

// renderNodePlist substitutes the placeholders scripts/node/install.sh substitutes, so what
// a test reads is what that script would install rather than the template. The values are a
// test's own: install.sh refuses anything sed or XML would read as syntax, so a plain path
// and a plain name are the whole range it passes through.
func renderNodePlist(t *testing.T, label, home string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("node", label+".plist"))
	if err != nil {
		t.Fatal(err)
	}
	rendered := strings.NewReplacer(
		"__CHECKOUT__", "/opt/localcode",
		"__MACHINE__", "config/machine-m5max-36gb.json",
		"__USER__", "server",
		"__HOME__", home,
		"__PATH__", "/opt/homebrew/bin:/usr/bin:/bin",
	).Replace(string(body))
	path := filepath.Join(t.TempDir(), label+".plist")
	if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A plist launchd cannot parse is a node that does not serve, and the file is written by a
// sed at install time: nothing between the template and /Library/LaunchDaemons reads XML.
// The serving daemon's KeepAlive is read as well, because launchd ORs the keys under it —
// SuccessfulExit left beside PathState would restart a server that was stopped on purpose.
func TestTheNodesPlistsRenderValidAndTheServerIsGatedOnTheSwitchAlone(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("plutil is macOS's, and so are the daemons these describe")
	}
	home := "/Users/server"
	for _, label := range []string{"com.localcode.link", "com.localcode.gpucap", "com.localcode.serve"} {
		path := renderNodePlist(t, label, home)
		if out, err := exec.Command("plutil", "-lint", path).CombinedOutput(); err != nil {
			t.Errorf("%s does not lint: %v\n%s", label, err, out)
		}
	}

	out, err := exec.Command("plutil", "-convert", "json", "-o", "-",
		renderNodePlist(t, "com.localcode.serve", home)).Output()
	if err != nil {
		t.Fatalf("reading the serving plist: %v", err)
	}
	var plist struct {
		ProgramArguments []string `json:"ProgramArguments"`
		KeepAlive        map[string]any
	}
	if err := json.Unmarshal(out, &plist); err != nil {
		t.Fatalf("serving plist: %v\n%s", err, out)
	}
	if len(plist.ProgramArguments) == 0 ||
		!strings.HasSuffix(plist.ProgramArguments[0], "/scripts/node/serve.sh") {
		t.Errorf("the daemon must run the switched wrapper, not the serving script: %v",
			plist.ProgramArguments)
	}
	if _, ok := plist.KeepAlive["SuccessfulExit"]; ok {
		t.Error("SuccessfulExit beside PathState: launchd ORs them, so a stopped server would be restarted")
	}
	state, ok := plist.KeepAlive["PathState"].(map[string]any)
	if !ok {
		t.Fatalf("KeepAlive is not gated on a path: %v", plist.KeepAlive)
	}
	if alive, ok := state[home+"/.local/state/localcode/serve.on"]; !ok || alive != true {
		t.Errorf("KeepAlive watches %v, not the serving user's switch", state)
	}
}

// The wrapper the daemon runs, which is what makes a stopped node stay stopped: launchd
// launches this job at load whatever the switch says, so the switch has to decide here.
func TestTheNodesServeWrapperServesOnlyWithTheSwitch(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "stub-server")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs(filepath.Join("node", "serve.sh"))
	if err != nil {
		t.Fatal(err)
	}
	switchPath := filepath.Join(dir, "serve.on")
	run := func() string {
		t.Helper()
		cmd := exec.Command(script)
		cmd.Env = append(os.Environ(), "SERVER_BIN="+stub, "SERVE_SWITCH="+switchPath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node/serve.sh: %v: %s", err, out)
		}
		return string(out)
	}

	if out := run(); !strings.Contains(out, switchPath) || strings.Contains(out, "serving config/node.env") {
		t.Errorf("with no switch the wrapper must start nothing and say why: %s", out)
	}
	if err := os.WriteFile(switchPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if out := run(); !strings.Contains(out, "serving config/node.env") {
		t.Errorf("with the switch there the node must serve its own config: %s", out)
	}
}

// Stopping the node's server is the switch's job wherever the installed plist is gated on
// it, because that route needs no root and the bootout does. The bootout stays for a node
// whose plist predates the switch, and a failed one is still a stop that did not happen.
func TestStopServerTakesTheSwitchRouteAndKeepsTheBootoutForAnOlderPlist(t *testing.T) {
	for _, c := range []struct {
		name        string
		switched    bool
		bootoutCode string
		wantCode    int
		wantSwitch  bool
		wantBootout bool
	}{
		{"a switched daemon is stopped by the file", true, "0", 0, false, false},
		{"a plist that predates the switch is booted out", false, "0", 0, true, true},
		{"a bootout that fails is not a stop", false, "1", 1, true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			bootoutLog := filepath.Join(dir, "bootout.log")
			stub := "#!/bin/sh\ncase \"$1\" in\n" +
				"print) exit 0 ;;\n" +
				"bootout) echo \"$@\" >> \"" + bootoutLog + "\"; exit " + c.bootoutCode + " ;;\n" +
				"esac\nexit 1\n"
			if err := os.WriteFile(filepath.Join(dir, "launchctl"), []byte(stub), 0o755); err != nil {
				t.Fatal(err)
			}
			switchPath := filepath.Join(dir, "serve.on")
			if err := os.WriteFile(switchPath, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			plist := filepath.Join(dir, "com.localcode.serve.plist")
			body := "<key>SuccessfulExit</key>"
			if c.switched {
				body = "<key>" + switchPath + "</key>"
			}
			if err := os.WriteFile(plist, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command("bash", "-c", ". ./lib.sh; stop_server")
			cmd.Env = append(os.Environ(),
				"PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
				// Wrong on purpose, as root's HOME makes it under sudo: the switch that is
				// removed has to be the one the installed plist names.
				"SERVE_SWITCH="+filepath.Join(dir, "roots-home", "serve.on"), "SERVE_PLIST="+plist,
				// Inherited, this would be evaluated: a test run from a measurement shell
				// must not stop that shell's server.
				"STOP_CMD=",
				// A name nothing on the machine running this answers to: what is under test
				// is which route was taken, and nothing here may kill a real server.
				"PROC=localcode-stop-test-no-such-process")
			out, err := cmd.CombinedOutput()
			if err != nil && cmd.ProcessState == nil {
				t.Fatalf("stop_server: %v", err)
			}
			if got := cmd.ProcessState.ExitCode(); got != c.wantCode {
				t.Errorf("exit %d, want %d: %s", got, c.wantCode, out)
			}
			if _, err := os.Stat(switchPath); (err == nil) != c.wantSwitch {
				t.Errorf("switch present = %v, want %v: %s", err == nil, c.wantSwitch, out)
			}
			if _, err := os.Stat(bootoutLog); (err == nil) != c.wantBootout {
				t.Errorf("bootout ran = %v, want %v: %s", err == nil, c.wantBootout, out)
			}
		})
	}
}
