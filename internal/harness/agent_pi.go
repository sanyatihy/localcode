package harness

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sanyatihy/localcode/internal/handoff"
)

// piAgent runs a chain's sessions in earendil-works/pi. Its configuration travels with the
// checkout: a provider registering the local endpoint, an extension holding the session to
// its budget, and a settings file that stops it compacting. All three are loaded by path,
// so a run does not depend on a machine having been set up by hand.
type piAgent struct {
	piRecorder
	bin      string
	root     string // the localcode checkout the extensions are loaded from
	model    string // what the provider file registers, so pi and the server name one thing
	maxOut   int    // the reservation that file declares, which the budget is taken from
	config   string // written by Prepare: the settings directory a session is pointed at
	selfPath string // this binary, which the gate extension runs per tool call
}

// The same four capabilities the incumbent's session gets, in Pi's spelling.
const piTools = "read,edit,write,bash"

func newPiAgent(root string) (Agent, error) {
	bin, err := exec.LookPath("pi")
	if err != nil {
		return nil, fmt.Errorf("pi is not on PATH: %w", err)
	}
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("could not find this binary to give the gate extension: %w", err)
	}
	model, maxOut, err := piDeclares(piProviderFile(root))
	if err != nil {
		return nil, err
	}
	return &piAgent{bin: bin, root: root, model: model, maxOut: maxOut, selfPath: self}, nil
}

func piProviderFile(root string) string {
	return filepath.Join(root, "harness", "pi", "local-provider.js")
}

func (p *piAgent) Name() string { return "pi" }

// piDeclares reads the two numbers the committed provider file declares about the model:
// the id pi must be given, and what one reply may generate. That file is the declaration
// pi acts on, so it is where they live — read rather than repeated here, because a second
// copy is a second answer to the same question. The context is in neither: the provider
// file reads that off the server.
func piDeclares(path string) (model string, maxTokens int, err error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", 0, fmt.Errorf("pi provider file not readable: %w", err)
	}
	id := regexp.MustCompile(`(?m)^\s*id:\s*"([^"]+)"`).FindSubmatch(body)
	out := regexp.MustCompile(`(?m)^\s*maxTokens:\s*(\d+)`).FindSubmatch(body)
	if id == nil || out == nil {
		return "", 0, fmt.Errorf("%s must declare a model id and maxTokens: a session is "+
			"budgeted from what the harness was told it may generate", path)
	}
	maxTokens, err = strconv.Atoi(string(out[1]))
	if err != nil {
		return "", 0, fmt.Errorf("%s: maxTokens is not a number: %w", path, err)
	}
	return string(id[1]), maxTokens, nil
}

// Window is the whole of what the server serves, and the file's own reservation. The
// provider file reads the context off `/props` at load, so what pi is told and what the
// server serves are one number by construction rather than by agreement.
func (p *piAgent) Window(served int) (int, int, error) {
	return served, p.maxOut, nil
}

// Prepare seeds the settings directory a session runs against. Pi reads `~/.pi/agent`
// unless told otherwise, which is the machine's rather than the checkout's — and what this
// file carries is a compaction reserve of 0, because a chain hands off rather than
// compacting.
func (p *piAgent) Prepare(chainDir string, oneShot bool) error {
	body, err := os.ReadFile(filepath.Join(p.root, "harness", "pi", "settings.json.reference"))
	if err != nil {
		return fmt.Errorf("pi settings not readable: %w", err)
	}
	dir := filepath.Join(chainDir, "pi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), body, 0o644); err != nil {
		return fmt.Errorf("could not write %s: %w", filepath.Join(dir, "settings.json"), err)
	}
	p.config = dir
	return nil
}

func (p *piAgent) Command(s Session) (string, []string, []string, error) {
	if p.config == "" {
		return "", nil, nil, errors.New("pi: no settings directory, so the session could " +
			"compact rather than hand off: Prepare must run before a session does")
	}
	args := []string{
		// Extension discovery off, and both of the checkout's loaded by path: what a
		// session runs with is then the checkout's, whatever the machine has installed.
		"-ne",
		"-e", piProviderFile(p.root),
		"-e", filepath.Join(p.root, "harness", "pi", "localcode-gate.js"),
		"--provider", "local", "--model", p.model,
		"--tools", piTools,
		"--append-system-prompt", s.Briefing,
		// Sessions are keyed by working directory under the config directory by default.
		// Told where to write, the driver knows the transcript's path before the session
		// starts rather than having to search for it afterwards.
		"--session-dir", s.StateDir,
	}
	if s.Goal != "" {
		// Events rather than a result: `pi -p` prints its answer once, at the end, which
		// on this machine is minutes of silence. The supervisor renders them, so it owns
		// every line the developer sees.
		args = append(args, "--mode", "json", "-p", s.Goal)
	}

	env := append(piEnv(), "PI_CODING_AGENT_DIR="+p.config,
		// Startup network operations are not the work: a session that waits on one spends
		// minutes before its first request, and this chain has nothing off the machine to
		// fetch.
		"PI_OFFLINE=1",
		// llama-server checks nothing, and the provider file names the variable rather
		// than a value so a real gateway could be dropped in without editing it.
		"LOCAL_OPENAI_API_KEY=local",
		// What the gate extension runs, once per tool call.
		"LOCALCODE_BIN="+p.selfPath,
		"LOCALCODE_HANDOFF_DIR="+s.StateDir)
	if s.Inherit != "" {
		env = append(env, "LOCALCODE_INHERIT="+s.Inherit)
	}
	return p.bin, args, env, nil
}

// piEnv is the parent's environment with pi's own variables dropped. A session launched
// from inside pi inherits its session id and its provider, which would point this one at
// another session's file.
func piEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if name, _, ok := strings.Cut(kv, "="); ok && strings.HasPrefix(name, "PI_") {
			continue
		}
		env = append(env, kv)
	}
	return env
}

// Writable is nothing: pi is told where its settings, its sessions and its state go, and
// all of them are under the chain's own directory, which the sandbox already opens.
func (p *piAgent) Writable() []string { return nil }

func (p *piAgent) Render(events io.Reader, out io.Writer, cwd string) string {
	return renderPiJSON(events, out, cwd)
}

// piRecorder finds the session file pi wrote. It is told where to write with
// `--session-dir`, so the file is in the directory the driver made for that session and
// nothing has to be searched for.
type piRecorder struct{}

// Cost is the largest context any turn reached and how many turns there were, read off the
// calls: pi records a usage per assistant message and nothing else has to be reconstructed.
func (r piRecorder) Cost(transcript string) (int, int) {
	calls, err := r.Requests(transcript)
	if err != nil {
		return 0, 0
	}
	peak := 0
	for _, c := range calls {
		if total := c.Ingest + c.Cached + c.Output; total > peak {
			peak = total
		}
	}
	return peak, len(calls)
}

// Requests reads pi's session file: one entry per message, and a `usage` on each assistant
// one. One entry is one call — pi files the whole assistant message as a single entry,
// content blocks and all, so nothing has to be folded back together the way the incumbent's
// transcript does.
//
// Latency is measured from the entry before the call, which is what the harness had
// finished when it sent it. A session file records no first-token time, so prefill, decode
// and whatever the harness spent between them arrive as one number — the same convention
// the incumbent's reader uses, so the two are comparable.
func (piRecorder) Requests(transcript string) ([]handoff.Request, error) {
	f, err := os.Open(transcript)
	if err != nil {
		return nil, fmt.Errorf("session file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var calls []handoff.Request
	var prev time.Time
	scan := bufio.NewScanner(f)
	// An entry holds a whole tool result. The default 64 KB would end the scan silently at
	// the first big one.
	scan.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scan.Scan() {
		var row struct {
			Type      string `json:"type"`
			Timestamp string `json:"timestamp"`
			Message   struct {
				Role  string `json:"role"`
				Usage struct {
					Input      int `json:"input"`
					Output     int `json:"output"`
					CacheRead  int `json:"cacheRead"`
					CacheWrite int `json:"cacheWrite"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(scan.Bytes(), &row); err != nil {
			continue // bookkeeping entries this does not model
		}
		at, err := time.Parse(time.RFC3339, row.Timestamp)
		if err != nil {
			continue // an entry with no clock cannot bound a call
		}
		if row.Type != "message" || row.Message.Role != "assistant" {
			if at.After(prev) {
				prev = at
			}
			continue
		}
		u := row.Message.Usage
		call := handoff.Request{Ingest: u.Input + u.CacheWrite, Cached: u.CacheRead, Output: u.Output}
		if !prev.IsZero() {
			call.Latency = at.Sub(prev)
		}
		calls = append(calls, call)
		prev = at
	}
	if err := scan.Err(); err != nil {
		return calls, fmt.Errorf("read %s: %w", transcript, err)
	}
	return calls, nil
}

func (piRecorder) Transcript(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return ""
	}
	// Newest by name, which is newest in time: the file is named for when the session
	// started. A session that was resumed leaves more than one.
	sort.Strings(names)
	return filepath.Join(dir, names[len(names)-1])
}
