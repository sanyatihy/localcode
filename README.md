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
brew install --cask claude-code@latest
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

### Serving from another Mac

Serve on the Mac with the memory, bound beyond loopback:

```sh
HOST=0.0.0.0 make serve CONFIG=config/agent.env
```

Committed configs keep `HOST="127.0.0.1"`; the environment overrides it for that run and
the banner prints what was bound. From the other Mac on the same trusted network, check
the endpoint and drive it:

```sh
ENDPOINT=http://mac.local:8081 make smoke
localcode -endpoint http://mac.local:8081 "fix the failing test"
```

Write that URL to `~/.config/localcode/endpoint` to make it this machine's default;
`-endpoint` still wins, and `localcode status` reports which was used. A remote endpoint is
started and stopped from here only when `~/.config/localcode/ssh` names a login for it, one
line of `user@host`: `localcode serve` and `localcode stop` then run that machine's own
launcher over SSH. With no such file they refuse and name it, and the commands to run are
`make serve` and `localcode stop` on the machine that serves. The session reaches loopback
and, through the launcher, that endpoint, and nothing else; port 8081 on this machine must be
free while the run lasts. Nothing authenticates, so serve only on a network you trust.

### Dedicated node

A Mac kept only to serve: no desk, lid closed, reachable by SSH, set up by running the
scripts below in order. Each one is re-runnable and skips what is already done. The node
here is an M5 Max with 36 GB on a USB cable to the laptop; it serves
[`config/node.env`](config/node.env) at 49,152 tokens.

**0. What macOS will not let a script do.** Do these once, at the machine:

- Plug the USB cable in and join the node to Wi-Fi, which is how it reaches the internet.
- Create the serving user, and turn on System Settings › General › Sharing › Remote Login
  for it. Nothing can enable the first login remotely, and a fresh account has no session.
- Copy your key: `ssh-copy-id <user>@<node>`. Password login is turned off in step 2, and
  a node with no key installed has no way back in.
- Turn on System Settings › General › Sharing › Remote Login › (i) › "Allow full disk
  access for remote users". Step 2's Remote Login levers need it, and root is not enough;
  for an SSH session the grant is on the node, not on the machine you connect from.
- Check `fdesetup status`. With FileVault on, no daemon starts and no key-only SSH login
  works until somebody unlocks the data volume after a reboot: either turn it off on a
  machine that only serves, or accept that a reboot needs a password before it serves.
- Give this laptop its end of the link, which no reboot or replug survives. From this
  checkout:

```sh
sudo MACHINE=config/machine.json ./scripts/node/install.sh --link-only
```

That installs `com.localcode.link` here, reading `en14` and `10.99.0.1` from
[`config/machine.json`](config/machine.json) and reapplying them every 30 seconds. For a
one-off instead, `sudo ifconfig en14 inet 10.99.0.1 netmask 255.255.255.0`, which is gone at
the next reboot or replug. `en14` is an `AppleUSBNCMData` interface with no `networksetup`
hardware port, so `networksetup` cannot manage it. The node's end, `anpi2` at `10.99.0.2`,
is installed by step 3. The cable negotiated USB 2 at 480 Mb/s; Thunderbolt Bridge is
inactive.

**1. Bootstrap, over SSH as the serving user.** Fetch the script on its own — there is no
checkout yet — read it, then run it:

```sh
curl -fsSLO https://raw.githubusercontent.com/sanyatihy/localcode/main/scripts/node/bootstrap.sh
sudo -v
LOCALCODE_NODE=1 bash bootstrap.sh
```

It installs the Command Line Tools without the GUI dialog, Homebrew, `go`, `python`,
`llama.cpp` and Claude Code, runs the `qwen35` architecture check, clones this repository
into `~/Developer/localcode` and runs `make install`. It prints what it did and what was
already there; a second run should say it did nothing. `LOCALCODE_REF` names the branch to
check out and defaults to `main`, so pass it on a node that follows a branch or the rerun
switches the checkout back. `sudo -v` first because it asks for
the password once, in your session, rather than installing a sudoers rule. It appends
Homebrew to `~/.zprofile`, which this shell has already read: open a new SSH session, or run
`exec zsh -l`, before step 2.

**2. Prepare**, from `~/Developer/localcode`:

```sh
sudo LOCALCODE_NODE=1 SERVE_USER=<user> ./scripts/node/prepare.sh
```

It turns off Spotlight, Siri, Apple Intelligence, Handoff, AirPlay, Bluetooth, Time Machine,
automatic updates, the screen saver, sleep on power and every sharing service but Remote
Login, restricts SSH to the serving user by key, and leaves Wi-Fi alone. It then prints what
macOS still keeps with nobody logged in and refuses a node reading above the record in
[`config/machine-m5max-36gb.json`](config/machine-m5max-36gb.json). Run it logged out at the
login window.

**3. Install the daemons:**

```sh
sudo LOCALCODE_NODE=1 SERVE_USER=<user> ./scripts/node/install.sh
```

`com.localcode.link` sets `anpi2` to `10.99.0.2` at every boot, `com.localcode.gpucap`
applies the GPU wired cap as root, and `com.localcode.serve` serves `config/node.env` as the
serving user bound to `0.0.0.0`. All three log to `/Library/Logs/localcode/`.

The serving daemon is gated on a switch file, `~/.local/state/localcode/serve.on` in the
serving user's home: launchd keeps the server up while that file is there and leaves it down
while it is not. `install.sh` creates it on a first install, and after that it is the node's
on-off switch — `./scripts/stop.sh` removes it and waits for the memory back, `localcode
serve` puts it back, and neither needs `sudo`, because the file belongs to the serving user.
It survives a reboot, so a node stopped on purpose comes up not serving.

What the node serves that the committed config does not goes in `~/.config/localcode/serve.env`
in the serving user's home, `KEY="value"` lines applied over `config/node.env`, for example
`CTX_SIZE="65536"`. The repository holds defaults; a setting one machine runs is that
machine's state. The daemon writes the merged file to `~/.local/state/localcode/serving.env`
and names it in its banner, so what is being served is one file a reader can open. Restart
with `localcode stop` and `localcode serve` to apply a change.

Every node script refuses without `LOCALCODE_NODE=1`, because each ruins a machine somebody
works at.

**4. Drive it from the laptop**, over the link:

```sh
ENDPOINT=http://10.99.0.2:8081 make smoke
localcode -endpoint http://10.99.0.2:8081 "fix the failing test"
```

Name the node's login once, beside the endpoint, and the laptop stops and starts it with no
password:

```sh
echo 'http://10.99.0.2:8081' > ~/.config/localcode/endpoint
echo '<user>@10.99.0.2' > ~/.config/localcode/ssh
localcode stop      # over SSH: removes the switch, waits for the node's memory back
localcode status    # nothing serving
localcode serve     # puts the switch back, waits for the model to answer
```

Both run the node's own `~/.local/bin/localcode` over `ssh -o BatchMode=yes`, so the key from
step 0 is what authorises them and a missing key is an error rather than a prompt. Without
`~/.config/localcode/ssh` they refuse: an endpoint somebody else shares is not this laptop's
to stop.

What this machine runs the harness with that the committed file does not goes in
`~/.config/localcode/claude-code.env`, applied over
[`harness/claude-code/claude-code.env`](harness/claude-code/claude-code.env): for example
`CLAUDE_CODE_MAX_OUTPUT_TOKENS="16384"` where the served model can afford a longer reply. The
repository holds defaults; a setting one machine runs is that machine's state.

The launcher's own settings go in `~/.config/localcode/localcode.env`, in the same form.
`RESULT_CAP_TOKENS="2560"` caps what one tool result may add to a session, which is what
sizes the reserve above its ceiling: derived, the reserve is a quarter of the window and
grows with it, so at 98,304 served the cap measured at 49,152 gives a 65,536-token ceiling
instead of 55,296.

The node is also on Wi-Fi at `<its Wi-Fi address>`. Nothing authenticates the endpoint — serve
only on a network you trust.

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
