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

Needs a Mac with Apple Silicon, Go (version in [`go.mod`](go.mod)), Python 3 for the probe
scripts, and `llama.cpp` from Homebrew — **build 10450**, the one every number here was
taken on. It must support the `qwen35` architecture, and this is the dependency that
breaks the model silently rather than loudly, so re-check after every `brew upgrade`:

```sh
strings /opt/homebrew/lib/libllama.dylib | grep -c '^qwen35$'   # expect 1
```

Then:

```sh
make check                              # the offline gate: Go, shell, doc links, race tests
make serve CONFIG=config/tuned.env      # llama-server on 127.0.0.1:8081, ~17 GB of weights
make smoke                              # in another shell: does it answer, and call a tool?
make eval N=3 LABEL=tuned-32k           # score the tier-1 suite and print the summary
make help                               # every target
```

The first `make serve` downloads ~17 GB into llama.cpp's own Hugging Face cache. `make
eval` refuses to start when the machine has less headroom than
[`config/machine.json`](config/machine.json) requires — a sweep that pages measures the
pager, not the model.

## Coding against the local model

`make install` puts `localcode` on your `PATH` with this checkout's location compiled in.
Then, in any repository:

```sh
localcode                          # start a server if none is running, and work here
localcode "fix the failing test"   # or give it one instruction and let it run
localcode sessions                 # the chains this repository has run
localcode -continue                # carry on the newest one
localcode status                   # what is being served, at what context
localcode stop                     # stop it, waiting for the memory back
```

Nothing is written to the repository you work in. Session state and the handoff live under
`~/.local/state/localcode/`, keyed by the repository's path, so two checkouts of one project
are two boxes of work.

**A session hands over instead of filling up.** Tool calls stop being permitted once the
session's context reaches a ceiling — the window less what a turn of results, the turn that
asked for them and the turn that writes the handoff still have to fit in — and the only
call still allowed is the one that writes the handoff — which the session cannot decline, because refusing a tool call is not a
request. So what you see when a session runs out is not an error, it is a line like:

```
session 1 — 11 tool calls, 7670 of 11264 tokens, 271s — next: apply the eight fixes, then run go test
```

and then a second session starting clean, knowing what the first learned and nothing else.
Given an instruction, `localcode` runs that chain for you until a handoff says `Next: none`,
until two sessions in a row plan the same step, or until `-sessions` runs out; interactively
it hands over once and leaves the next move to you, which is `localcode -continue`.

Resuming re-reads nothing: a handoff costs about **4,265 tokens and 54 s**, where compacting
the conversation it replaces re-read **37,837 tokens** — 452 s of ingest on this machine
before a word of summary. That is why compaction is refused here rather than tuned, and why
`--resume` is not what `-continue` does.

`-resume <id>` picks a chain by name and `-fork <id>` starts a new one from what that chain
knew, so a second attempt does not have to relearn the first. `-ceiling`, `-calls`, `-sessions` and
`-session-timeout` move the bounds; the ceiling defaults to as much as the arithmetic
allows, so `-ceiling` only lowers it.

**The agent is sandboxed, which is what makes an unrestricted `Bash` tool defensible.**
Writes reach the working directory, temp and the cache roots; everything else the kernel
refuses. Reads are unrestricted, because an agent that cannot read a toolchain cannot use
one. The network is loopback-only, so a repository's source cannot leave the machine —
`localcode -net` opens it for one session when something has to be installed. A refused
write names the path and the line that allows it:

```sh
echo ~/.cargo >> ~/.config/localcode/writable
```

**`agent.env`, never `tuned.env`:** it serves the sampling, thinking toggle and
chat-template override Claude Code never sends itself. `localcode` serves it by default.

Moving this checkout means running `make install` again — the path is compiled in, not
searched for.

## How the repo is laid out

```
cmd/        the six commands, each with its exit codes documented in its package comment
internal/   the packages behind them: eval (scoring), harness (adapters), prefix, handoff
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
| [`cmd/handoff`](cmd/handoff) | run one task box across fresh sessions instead of compacting |
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
