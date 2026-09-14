package scripts

import (
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
	// 32 GiB installed, 16,384 MiB already capped by hand, and the same value asked for
	// again: the state a node is in when its serving daemon restarts after the boot-time
	// cap has already been applied.
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
