package scripts

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
