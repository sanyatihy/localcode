# localcode

Run a coding agent on your MacBook: **Qwen3.8-27B Q4_K_M**, served by llama.cpp
and driven by Claude Code by default. Model inference stays local. Measurements
come from one **M2 Max with 32 GB**, not Apple Silicon generally.

## Getting started

Use an **Apple Silicon Mac with at least 32 GB unified memory** for this walkthrough;
smaller-memory and Intel Macs are untested. Budget **25 GB of free disk space**
for roughly 17 GB of weights plus tools and caches. Stay online through installation
and the first download, and close memory-heavy apps before loading the model.

### 1. Install dependencies

In **Terminal** (zsh), install Apple's Command Line Tools and finish the dialog
before continuing. Skip this if already installed:

```sh
xcode-select --install
```

Install [Homebrew](https://brew.sh/) if needed, then enable it in this and future
terminals and install the dependencies:

```sh
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> ~/.zprofile
eval "$(/opt/homebrew/bin/brew shellenv)"
brew install go python llama.cpp
brew install --cask claude-code
```

[Claude Code's native package](https://code.claude.com/docs/en/quickstart) needs no
Node.js. Go may download the toolchain specified in [go.mod](go.mod) on first build.
Homebrew versions can differ from the [measured versions](docs/TECH.md#dependencies);
after installing or upgrading llama.cpp, check for a nonzero architecture count:

```sh
strings "$(brew --prefix)/lib/libllama.dylib" | grep -c '^qwen35$'
```

### 2. Install Localcode

```sh
mkdir -p ~/Developer
cd ~/Developer
git clone https://github.com/sanyatihy/localcode.git
cd localcode
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zprofile
export PATH="$HOME/.local/bin:$PATH"
make install
```

This builds `~/.local/bin/localcode` with the checkout's path compiled in.
Keep the checkout; rerun `make install` after moving or updating it.

### 3. Download and test

From the checkout, start the first download in the foreground to see progress:

```sh
make serve CONFIG=config/agent.env
```

Keep that terminal open while roughly 17 GB downloads from Hugging Face and loads.
In a **second Terminal**, run:

```sh
cd ~/Developer/localcode
curl -fsS http://127.0.0.1:8081/health
```

Once health returns `{"status":"ok"}`, check a reply and tool call:

```sh
make smoke
localcode status
```

Expect `smoke passed` and the Qwen model at **49152 ctx**. Then try a small task:

```sh
mkdir -p ~/Developer/localcode-tryout
cd ~/Developer/localcode-tryout
git init
localcode -sessions 3 "Create hello.py that prints Hello from localcode, run it with python3, and finish."
python3 hello.py
localcode stop
```

Success is `Hello from localcode` printed by the created file. Allow a few minutes.
This configured flow needs no Anthropic API key or paid Claude subscription:
`localcode` supplies the local endpoint and placeholder token.

On later runs, `localcode` starts the server automatically. `localcode stop` shuts
it down and waits for memory to be released.

### Troubleshooting

- **Command not found:** reopen Terminal and check
  `command -v brew go python3 llama-server claude localcode`.
- **Startup fails or health is not ready:** check the server terminal, or
  `tail -n 80 ~/.local/state/localcode/serve.log` for automatic startup.
  Automatic startup waits 20 minutes; use the foreground download on slow connections.
- **Wrong context or port busy:** stop the existing server, then start
  `config/agent.env` again. If another application owns port 8081, stop it first.
- **Sluggish desktop or Metal allocation failure:** run `localcode stop` and close
  memory-heavy apps.
- **Still failing:** include the command/error, chip/RAM, `sw_vers`,
  `claude --version` and `llama-server --version` in your report.

## Coding against the local model

Run from the repository you want the agent to edit:

```sh
localcode                         # interactive session
localcode "fix the failing test"  # sessions run back to back
localcode sessions                # list this repository's chains
localcode -continue               # continue the newest chain
localcode -resume <id>            # continue a specific chain
localcode -fork <id>              # start from an earlier handoff
localcode account                 # report the latest chain's cost
localcode -h                      # all commands and flags
```

Session state lives under `~/.local/state/localcode/`, keyed by repository path.
The agent edits your project files; its handoffs stay outside the project.

### One instruction, run until it is done

Each session has a context and tool-call budget. At its limit, it writes a handoff;
the next session reads that handoff and the original instruction. Interactive
sessions wait for you to run `-continue`; an instruction starts subsequent sessions
automatically. This replaces compaction: see
[session handoffs](docs/TECH.md#sessions-hand-off-instead-of-compacting).

| exit | meaning |
|---|---|
| 0 | the chain reports completion, or the interactive session exits cleanly |
| 1 | unfinished: a repeated next step, session limit, timeout, interruption or agent failure |
| 2 | setup or invocation failure |

An unfinished chain names its handoff; inspect it before resuming. Verify the
resulting changes and tests even when the agent reports completion.

| flag | default | purpose |
|---|---|---|
| `-sessions <n>` | 60 | maximum sessions per instruction |
| `-session-timeout <d>` | 1h | time limit per session; off interactively |
| `-ceiling <pct>` | maximum that fits | lower the context budget |
| `-calls <n>` | derived from context | override the tool-call budget |

### Configuration and sandbox

The default [agent config](config/agent.env) serves **49,152 tokens**, thinking off,
model-card sampling and the Claude Code template override. The optional
[MTP config](config/driver-mtp-32k.env) uses 32,768 tokens and requires a llama.cpp
build carrying the MTP head. Stop the server before changing configs; an existing
server is reused. See [serving measurements](docs/TECH.md#serving) and
[harness comparisons](harness/README.md) for alternatives.

The agent's network is loopback-only; `-net` allows outbound access for that run.
Writes are limited to the project, its named sibling worktrees, temporary files,
caches and agent state. Reads exclude credential directories such as `~/.ssh`.
Additional writable paths go in `~/.config/localcode/writable`, one per line.
See [the sandbox boundary](docs/TECH.md#the-local-model-runs-in-any-repository-sandboxed).

## Checks and benchmarks

From the Localcode checkout:

```sh
brew install shellcheck
make check
make help
```

The gate needs no model, but downloads Go tools on a cold run and uses the
vulnerability database. Keep network access available.

For tier-1 scoring, use `config/tuned.env`: the scorer supplies its own sampling
settings. This config is for evaluation; use `config/agent.env` for coding.

```sh
localcode stop
make serve CONFIG=config/tuned.env
```

In another terminal, from the checkout:

```sh
make eval N=3 LABEL=tuned-32k
```

Evaluation checks [machine limits](config/machine.json) recorded for the measured
laptop. Results and their limits live in [TECH](docs/TECH.md); raw rows are in
[docs/data](docs/data/).

## Repository and contributing

| directory | contents |
|---|---|
| [cmd](cmd) | Localcode launcher, scoring and reporting commands |
| [internal](internal) | harness adapters, session budgets and measurement code |
| [config](config) | serving profiles and machine limits |
| [scripts](scripts) | serving, checks and measurement scripts |
| [tasks](tasks) | scoring fixtures |
| [harness](harness) | agent setup and comparisons |
| [runtimes](runtimes) | alternative runtime experiments |
| [docs](docs) | vision, technical findings, feature plans and data |

Read [VISION](docs/VISION.md) for scope, [TECH](docs/TECH.md) for as-built facts and
[gotchas](docs/TECH.md#gotchas), and [AGENTS.md](AGENTS.md) for the work protocol.
Feature plans and history are in [docs/features](docs/features/).
Changes go through human-reviewed PRs; submitted feature docs on trunk are frozen.

The protocol uses `kit`, a separate private tool. Installing, running and testing
Localcode does not require it; `make check` is the repository gate used by CI.
