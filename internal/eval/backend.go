package eval

import (
	"os/exec"
	"strings"

	"github.com/sanyatihy/localcode/internal/build"
)

// The Metal kernels are not llama.cpp's own. ggml ships them in a separately versioned
// library the server opens at load, so two runs reporting one build_info can decode with
// different kernels — which is how sparse flash attention arrived without llama.cpp's
// version moving for it.

// Backend answers which ggml library served a run. A package variable so a test can
// answer for a machine of its own, as Sample does for memory.
var Backend = resolveBackend

// resolveBackend names the ggml backend library open in the process listening on
// endpoint, and build.Unknown when it cannot be read. Unknown rather than a guess: a
// wrong library here attributes a kernel change to the wrong build.
func resolveBackend(endpoint string) string {
	port := portOf(endpoint)
	if port == "" {
		return build.Unknown
	}
	out, err := exec.Command("lsof", "-nP", "-iTCP:"+port, "-sTCP:LISTEN", "-t").Output()
	if err != nil {
		return build.Unknown
	}
	pid := strings.TrimSpace(strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0])
	if pid == "" {
		return build.Unknown
	}
	files, err := exec.Command("lsof", "-p", pid, "-Fn").Output()
	if err != nil {
		return build.Unknown
	}
	return backendFrom(string(files))
}

// portOf reads the port off an endpoint URL. Anything else is a caller error rather than
// a machine that could not answer, and both reach the row as unknown.
func portOf(endpoint string) string {
	s := endpoint
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?"); i >= 0 {
		s = s[:i]
	}
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return ""
	}
	port := s[i+1:]
	if port == "" || strings.ContainsAny(port, "]") {
		return ""
	}
	return port
}

// backendFrom picks the ggml backend out of `lsof -Fn` output. libggml-base is the shared
// half every build loads and names no kernels, so it is not the answer.
func backendFrom(lsofOutput string) string {
	for _, line := range strings.Split(lsofOutput, "\n") {
		if !strings.HasPrefix(line, "n/") {
			continue
		}
		path := line[1:]
		base := path[strings.LastIndex(path, "/")+1:]
		if !strings.HasPrefix(base, "libggml-") || strings.HasPrefix(base, "libggml-base") {
			continue
		}
		if strings.HasSuffix(base, ".so") || strings.HasSuffix(base, ".dylib") {
			return path
		}
	}
	return build.Unknown
}
