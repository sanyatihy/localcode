# localcode

Agentic coding on one MacBook, with the model on the machine: **Qwen3.8-27B** at Q4_K_M,
served by llama.cpp, driven by Claude Code. Every choice here was settled by measurement, and
every number was taken on one **M2 Max with 32 GB** — nothing here is a benchmark claim about
Apple Silicon in general.

- [`docs/VISION.md`](docs/VISION.md) — what this is for, and what it is not.
- [`docs/TECH.md`](docs/TECH.md) — **the answers**, as built and as measured.
- [`docs/features/`](docs/features/) — how each answer was arrived at. Every one has shipped
  or been dropped; a shipped doc is frozen history.
- [`docs/data/`](docs/data/) — the raw rows every table rests on.
- [`docs/TECH.md#gotchas`](docs/TECH.md#gotchas) — every trap that has already cost this repo
  a wrong number. Worth reading before changing how anything is measured.

## What was settled

| question | answer | where the numbers are |
|---|---|---|
| runtime | **llama.cpp**, not MLX | [llama.cpp against MLX](docs/TECH.md#llamacpp-against-mlx) |
| model and quant | Qwen3.8-27B `Q4_K_M` | [the measured envelope](docs/TECH.md#the-measured-envelope) |
| KV cache | `q8_0` for K and V | same |
| context | **32k** to grind, **49k** for the editor | [serving](docs/TECH.md#serving) |
| thinking | **off** — 4× the wall clock bought nothing | [sampling and thinking](docs/TECH.md#sampling-and-thinking-settled) |
| sampling | the model card's own pair for thinking-off | same |
| harness | **Claude Code** stays; nothing displaced it | [nothing displaces Claude Code](docs/TECH.md#nothing-displaces-claude-code-and-the-two-axes-disagree) |
| decode speedup | the model's own MTP head, **at 32k only** | [speculative decoding](docs/TECH.md#speculative-decoding-adoptable-at-the-top-of-the-context-not-the-bottom) |

Two limits shape every one of those. **Ingest time binds before memory does** — a cold 64k
context costs 13.1 minutes — and **the desktop fails before the model does**: 65,536 leaves
the machine unusable while every request still succeeds, so the ceiling for a machine
somebody is using is 57,344.

## Getting started

This is the path from a clean MacBook to a first coding task. Use **an Apple Silicon
Mac with 32 GB or more unified memory** for this walkthrough; the measured machine is
an M2 Max with 32 GB. Smaller-memory Macs and Intel Macs are not validated. Close
memory-heavy apps before loading the model. Allow **at least 25 GB of free disk space**
as a setup budget for roughly 17 GB of weights plus tools and caches, and stay online
for installation and the first download. Later model inference runs locally.

### 1. Install the tools

Open **Terminal** (the commands below assume macOS's default zsh). Install Apple's
Command Line Tools, which provide Git, make and the compiler:

```sh
xcode-select --install
```

Finish the installer dialog before continuing. “Already installed” is fine.
Install [Homebrew](https://brew.sh/) if you do not already have it:

```sh
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

Follow the installer's **Next steps** to add Homebrew to your shell. On Apple Silicon
the standard setup is:

```sh
echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> ~/.zprofile
eval "$(/opt/homebrew/bin/brew shellenv)"
brew install go python llama.cpp
brew install --cask claude-code
```

[Claude Code's Homebrew installation](https://code.claude.com/docs/en/quickstart)
provides the `claude` command; Node.js is not needed for this installation method.
Python is used by the session hooks as well as the check scripts. Go's required
version and toolchain are in [go.mod](go.mod); Go may download that toolchain on the
first build. Check that all three commands resolve:

```sh
go version
python3 --version
claude --version
llama-server --version
```

Homebrew installs its current releases, which may differ from the versions measured
here. See [runtime versions and compatibility](docs/TECH.md#dependencies) and the
[Claude Code setup](harness/claude-code/README.md). After installing or upgrading
llama.cpp, check its model architecture support:

```sh
strings "$(brew --prefix)/lib/libllama.dylib" | grep -c '^qwen35$'
```

Expect a nonzero count. A zero or missing library needs investigation before loading
weights; passing this check alone does not prove the whole flow works.

### 2. Clone and install Localcode

Keep this checkout somewhere permanent: its path is compiled into the launcher.

```sh
mkdir -p ~/Developer
cd ~/Developer
git clone https://github.com/sanyatihy/localcode.git
cd localcode
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zprofile
export PATH="$HOME/.local/bin:$PATH"
make install
localcode -h
```

`make install` builds `~/.local/bin/localcode`. If you move the checkout or pull an
update, run `make install` there again. You do not need `kit` to install or use it.

### 3. Download the model and check the server

For the first download, run the server in the foreground so progress and errors are
visible. Run this from `~/Developer/localcode`:

```sh
make serve CONFIG=config/agent.env
```

This downloads roughly 17 GB from Hugging Face into llama.cpp's cache and then loads
the model. Keep this terminal open. Download time depends on your connection; loading
and the first reply can also take time. The endpoint is `127.0.0.1:8081`.

In a **second Terminal window**, wait until the health request returns
`{"status":"ok"}`, then run the smoke test:

```sh
cd ~/Developer/localcode
curl -fsS http://127.0.0.1:8081/health
make smoke
localcode status
```

Success is `smoke passed` and status reporting the Qwen model at **49152 ctx**.
A connection refusal or a loading response means it is not ready yet; check the
first terminal. The smoke test checks both a reply and a tool call.

### 4. Give it a small coding task

In the second terminal, create a disposable repository and ask for an observable edit:

```sh
mkdir -p ~/Developer/localcode-tryout
cd ~/Developer/localcode-tryout
git init
localcode -sessions 3 "Create hello.py that prints Hello from localcode, run it with python3, and finish."
python3 hello.py
```

Success is a file created by the agent and `Hello from localcode` printed when you run
it yourself. Allow a few minutes for the first task on the measured machine.
Localcode supplies the local endpoint and a placeholder authentication token itself;
this configured flow does not need an Anthropic API key or paid Claude subscription.
Launch it with `localcode`; no manual environment sourcing or cloud login is part of
this walkthrough.

When finished:

```sh
localcode stop
```

This stops the server and waits for memory to be released. On subsequent runs,
`localcode` starts the server automatically with `config/agent.env`; you do not need
the separate server terminal once the weights are cached.

### If something goes wrong

- **Command not found:** reopen Terminal after the PATH steps, then check
  `command -v brew go python3 llama-server claude localcode`.
- **Download or startup fails:** read the foreground server output. For a server
  started automatically, use `tail -n 80 ~/.local/state/localcode/serve.log`.
  Automatic startup waits up to 20 minutes; use the foreground download above on
  a slow connection.
- **Port 8081 is busy or status reports another context:** stop the existing Localcode
  server with `localcode stop`, then start `config/agent.env` again. If another
  application owns the port, stop that application first.
- **The desktop becomes sluggish or Metal reports allocation failure:** stop the
  server and close memory-heavy apps. The supplied settings were measured on one
  32 GB M2 Max; other machines are not a reproduced result.
- **The agent fails while smoke passes:** record `claude --version`,
  `llama-server --version`, `sw_vers`, your chip/RAM, the command and its error.
  Newer dependency versions can change behavior; the smoke test checks the server,
  while the small coding task checks the whole launcher/harness path.

### Optional: checks and benchmarks

From `~/Developer/localcode`, `make check` runs the repository gate without loading
a model. Install `shellcheck` for the full shell check. A cold run downloads Go tools,
and vulnerability checking uses the advisory database, so keep network access available.

```sh
brew install shellcheck
make check
make help
```

To reproduce tier-1 scoring, stop the agent server and serve the scorer's config in
one terminal, then evaluate in another:

```sh
localcode stop
make serve CONFIG=config/tuned.env
```

```sh
cd ~/Developer/localcode
make eval N=3 LABEL=tuned-32k
```

The scorer supplies its own sampling settings; use `config/agent.env` again before
coding. Evaluation checks the headroom limits in
[config/machine.json](config/machine.json), which describe the measured laptop.

## Coding against the local model

After the setup above, run these from any repository:

```sh
localcode                          # a session here, with you at the keyboard
localcode "fix the failing test"   # one instruction, run until it is done
localcode status                   # what is being served, at what context
localcode stop                     # stop it, waiting for the memory back
```

The agent edits files in the repository you work in. Its session state and handoffs live under
`~/.local/state/localcode/`, keyed by the repository's path, so two checkouts of one project
are two boxes of work.

**Whichever mode you use, a session hands over instead of filling up.** A 27B model on this
machine has a small window, and a session that reaches the end of it dies holding everything
it learned. So every session is given a budget; when it runs out, every tool call is refused
except the one that writes a handoff, and the next session starts clean with that handoff and
nothing else. The two modes differ only in who starts that next session.

### With you at the keyboard

`localcode` with no instruction is an ordinary Claude Code session — its own screen, its own
prompt, ended when you end it. When it reaches its budget it says so and stops being able to
work. A handoff is written either way: by the session, or out of its transcript when it does
not. Pick it up where it left off:

```sh
localcode -continue
```

That starts a **fresh** session that has read the handoff. It has not re-read the
conversation, which is the whole point — see [why not compaction](#why-a-handoff-and-not-a-summary).

### One instruction, run until it is done

`localcode "…"` starts a **chain**: sessions back to back, each inheriting the last one's
handoff, with the instruction re-issued word for word so it cannot drift. You watch it work
— the model's reasoning arrives a token at a time as it is written, and every call and every
refusal as it happens:

```
  → Read median.go
  → Edit median.go
  → Bash go test ./...
  ✗ this session's context reached 7280 tokens of a 7168 ceiling. No call other than…
  → Write HANDOFF.md
session 1 — 14 tool calls, 9123 of 12288 tokens, 453s — next: apply the six remaining fixes
```

The `✗` is the budget doing its job, not an error. The last line is the summary of a session
that has finished; then a new one starts. A chain ends in one of four ways, and says which:

| what happened | exit | what to do |
|---|---|---|
| a handoff says `Next: none` | 0 | nothing — the work is done |
| two sessions in a row plan the same step | 1 | read the handoff it names |
| a session ran past `-session-timeout` | 1 | read the handoff it names |
| `-sessions` ran out | 1 | `localcode -resume <id>` to carry on |

Measured on eight independent bugs in eight files, at a deliberately small window: three
sessions, 999 s, all eight tests passing, with the first two sessions both cut off mid-work.
The rows are in
[`docs/data/`](docs/data/2026-08-22-m2max-32gb-0023-chain.jsonl).

### Picking up an earlier chain

A repository can hold as many chains as you have given it instructions. Starting fresh is the
default, which is what `claude` does too:

```sh
localcode sessions          # every chain here: id, sessions, how it ended, where it got to
localcode -continue         # carry on the newest one
localcode -resume <id>      # carry on that one
localcode -fork <id>        # start a new chain from what that one knew
```

### The bounds you can move

| flag | default | what it does |
|---|---|---|
| `-ceiling <pct>` | as much as fits | how full a session may get before it hands over. Only lowers |
| `-calls <n>` | derived | tool calls one session may spend, overriding what its ceiling implies |
| `-sessions <n>` | 60 | sessions one instruction may take |
| `-session-timeout <d>` | 1h | how long one session may run. Off when you are at the keyboard |

The ceiling defaults to as much as the arithmetic allows, so `-ceiling` only ever lowers it —
and lowering it costs a handoff every time it cuts a session short. Reach for `-ceiling 20` to
watch a handover happen on work that would otherwise fit in one session.

**Two tools are capped so that one call cannot spend a session.** A command's output is
truncated, and a `Read` of a long file comes back as a slice rather than the whole thing — ask
again with an offset for more. Both caps are sized from the window, so they widen with it.

### Why a handoff and not a summary

Resuming re-reads nothing. A handoff costs about **4,265 tokens and 54 s**, where compacting
the conversation it replaces re-read **37,837 tokens** — 452 s of ingest on this machine before
a word of summary, and it was measured losing the goal it was summarising. So compaction is
refused here rather than tuned, and `-continue` is not `claude --resume`, which would re-ingest
the conversation that had just failed to fit.

**The agent is sandboxed, which is what makes an unrestricted `Bash` tool defensible.**
Writes reach the working directory — `.worktrees/` included — plus worktrees beside it named
after it (`repo-0001` next to `repo`, which is what `git worktree add ../repo-0001` needs),
temp and the cache roots; everything else the kernel refuses. The session is told which of
the two places a worktree may go, since it cannot find out except by being refused. Reads
are open, because an agent that cannot read a toolchain cannot use one — apart from
`~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.config/gh` and `~/Library/Keychains`, which hold
credentials and answer `Operation not permitted`. The network is loopback-only, so a repository's source cannot leave the machine —
`localcode -net` opens it for one session when something has to be installed. A refused
write names the path and the line that allows it:

```sh
echo ~/.cargo >> ~/.config/localcode/writable
```

Each path that file opens is printed at startup, and a line that would open a credential
root is refused rather than applied.

**Never `tuned.env`:** Claude Code sends no sampling, no thinking toggle and no
chat-template override, so a config that does not serve all three serves something nobody
chose. `localcode` starts [`config/agent.env`](config/agent.env) by
default — those defaults at 49,152, on the binary on your `PATH`.
`-config config/driver-mtp-32k.env` is the same defaults at 32,768 plus the model's own MTP
head, which the allocator admits there and refuses at 49,152; it needs the llama.cpp build
that carries the head, and says so rather than hanging if it is missing. That config decodes
1.38x faster and is the better one for work whose calls are cheap, but on real source a
10,240 ceiling ends every session before its call budget, so the default is the larger one.

Moving this checkout means running `make install` again — the path is compiled in, not
searched for.

## How the repo is laid out

```
cmd/        the five commands, each with its exit codes documented in its package comment
internal/   the packages behind them: eval (scoring), harness (adapters), prefix, transcript
config/     one .env per serving config, plus machine.json — every limit this laptop imposes
scripts/    the shell around a measurement: serve, ladder, screen, pair, probe
tasks/      the fixtures the model is scored on; tasks/depth/ is the deep floor check
harness/    how each coding agent is pointed at the local endpoint
runtimes/   MLX, which lost to llama.cpp and is kept because a comparison must be re-runnable
docs/       vision, as-built facts, feature history, and the raw data
results/    live scratch, gitignored; snapshots land in docs/data/ when a feature ships
```

### The commands

| command | what it does |
|---|---|
| [`cmd/eval`](cmd/eval) | tier 1 — one request per fixture, scored deterministically |
| [`cmd/report`](cmd/report) | summarise a results file; `-baseline` reports the rest against one |
| [`cmd/tier2`](cmd/tier2) | tier 2 — drive a whole agent loop through a fixture in a scratch checkout |
| [`cmd/prefixprobe`](cmd/prefixprobe) | replay a fixed conversation and record what each turn was charged |
| [`cmd/prefixlog`](cmd/prefixlog) | the same accounting for traffic nobody scripted, off the server's log |

Every command documents its exit codes at the top of `main.go`, because the scripts branch
on them rather than parsing output.

## Contributing

[`AGENTS.md`](AGENTS.md) is the work protocol — feature docs, one branch per feature, and
what may be written where. Two rules matter before anything else: a doc marked
`status: Shipped` is frozen history and is never edited, and no change reaches `main`
except through a pull request.

It is written by `kit`, a separate private tool that also runs the board (`kit next`) and
the drift check (`kit audit`). **Nothing in this repo needs it** — `make check` is the gate,
and CI runs that alone — but the protocol's commands will not resolve without it.
