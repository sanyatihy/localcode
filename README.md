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

1.  **Serve, in this checkout.** `agent.env`, never `tuned.env`: it serves the sampling,
    thinking toggle and chat-template override Claude Code never sends itself.

    ```sh
    make serve CONFIG=config/agent.env   # first run downloads ~17 GB; make smoke checks it
    ```

2.  **Add this to `~/.zshrc`.** Use the absolute path to this checkout.

    ```sh
    localclaude() (               # parens make it a subshell, so nothing leaks into yours
      set -a; . ~/src/localcode/harness/claude-code/claude-code.env; set +a
      exec claude --tools Bash,Edit,Read,Write --allowedTools Bash,Edit,Read,Write "$@"
    )
    ```

3.  **Run `localclaude` in any repository.** Nothing is written to it, to your shell, or to
    `~/.claude`.

4.  **Stop with `make stop`.** It waits for the memory back rather than only killing.

`--tools` cuts the preamble from 18,388 tokens to 3,711 and `--allowedTools` pre-approves that
same set, which is what runs it unprompted — `--permission-mode auto` calls home for its Bash
check and `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` blocks it, while `dontAsk` denies rather
than allows.

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
