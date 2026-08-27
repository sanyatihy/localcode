package harness

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/sanyatihy/localcode/internal/handoff"
)

// Agent is one coding agent a chain can run its sessions in. Everything that differs
// between agents is behind it: the flags a session takes, the environment it needs, the
// window it must be told it has, where it records what the session cost, and the shape of
// the events it prints while it works.
//
// The driver decides what a session may spend and what it is told; this decides how to say
// that to one particular agent. Nothing above it names one.
type Agent interface {
	Recorder
	Name() string

	// Window is what the agent will be told it has, given what the endpoint serves. Both
	// numbers, because a session's budget is derived from the pair: the reply shares the
	// served context with the conversation, and what it reserves is the agent's own.
	Window(served int) (declared, output int, err error)

	// Prepare is what one chain needs written before its first session. Called once, with
	// the chain's directory. Whether a session may be refused permission to stop is not
	// here: it is one decision, it is the supervisor's, and it travels in the spec.
	Prepare(chainDir string) error

	// Command is the binary, the arguments and the environment for one session. The
	// sandbox is the driver's, so what comes back is what goes inside it.
	Command(s Session) (bin string, args, env []string, err error)

	// Writable is where the agent keeps its own state, which the sandbox has to let it
	// write. The boundary is the driver's and the paths are the agent's: a session denied
	// its own history directory fails in a way that reads like a model failure.
	Writable() []string

	// Render turns one session's event stream into the lines a person watching sees, and
	// returns the server's own refusal when a prompt went past what it serves.
	Render(events io.Reader, out io.Writer, cwd string) (overrun string)
}

// Recorder is where a finished session recorded what it cost. Separate from the rest
// because reading a chain back needs only this: `localcode account` runs long after the
// session did, from a directory that is not a checkout and with no agent to start.
type Recorder interface {
	// Transcript is the file the finished session recorded itself in, or "" when it left
	// none. `dir` is the session's own directory, which is the only thing a reader knows
	// about a session after it has ended.
	Transcript(dir string) string

	// Cost is what that record says the session spent: the largest context any turn
	// reached, and how many turns it took. Peak is -1 when there is nothing to read,
	// because a peak is tested against a ceiling and a 0 there is a session that spent
	// nothing rather than one nobody could measure.
	Cost(transcript string) (peak, turns int)

	// Requests is every call the session made to the model, in order. What one cost and
	// how long it took is in the record and in nothing else, so a chain's rates are read
	// from here.
	Requests(transcript string) ([]handoff.Request, error)
}

// Session is one session's worth of what the driver knows. Paths rather than contents,
// because an agent reads and writes them itself.
type Session struct {
	// Goal is the instruction, and empty means a developer at a keyboard: an agent given
	// no instruction owns the terminal and ends when they end it.
	Goal string
	// Briefing is appended to the agent's system prompt. It is the driver's words about
	// the sandbox, the handoff and what this chain has learned — trusted because it
	// arrives as instruction rather than through a tool result.
	Briefing string
	// StateDir is this session's own directory: its budget, its counters, the handoff it
	// is kept with afterwards.
	StateDir string
	// StableDir is the chain's directory, which is where the handoff is read from and
	// written to. The same path for every session of a chain, so the preamble is too.
	StableDir string
	// Inherit is the handoff the session before it wrote, or "" for the first.
	Inherit string
	// ResultCap is what one tool result may add to the context, in bytes.
	ResultCap int
}

// DefaultAgent is the harness a chain runs in when nothing says otherwise: the incumbent,
// which 0010 measured as the one nothing displaces on both axes at once.
const DefaultAgent = "claude-code"

// agents is every agent this package drives, and the whole of what the rest of the
// project may choose between. A third one is added here and in its own file, and nowhere
// else — which is the property `TestNoPackageAboveTheAdaptersNamesAHarness` holds.
var agents = map[string]struct {
	agent    func(root string) (Agent, error)
	recorder func() Recorder
}{
	"claude-code": {newClaudeCodeAgent, func() Recorder { return &claudeCodeRecorder{} }},
	"pi":          {newPiAgent, func() Recorder { return piRecorder{} }},
}

// Names is every agent there is, for a reader choosing between them.
func Names() string {
	names := make([]string, 0, len(agents))
	for name := range agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// NewAgent returns the agent that name asks for, configured from a localcode checkout.
// It refuses rather than falling back, and names what it drives: a chain run against a
// different agent from the one asked for is a comparison of nothing.
func NewAgent(name, root string) (Agent, error) {
	if a, ok := agents[name]; ok {
		return a.agent(root)
	}
	return nil, unknown(name)
}

// NewRecorder returns just the reader for one agent, which is what a chain read back after
// the fact needs.
func NewRecorder(name string) (Recorder, error) {
	if a, ok := agents[name]; ok {
		return a.recorder(), nil
	}
	return nil, unknown(name)
}

func unknown(name string) error {
	return fmt.Errorf("no harness %q: localcode drives %s", name, Names())
}
