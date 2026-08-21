# localcode

Making one MacBook a viable place to run agentic coding: a quantised **Qwen3.8-27B**
served locally, tuned by measurement rather than guesswork, driven by a coding harness
chosen on evidence. The work here is **how the model is launched, served and driven** —
not the weights, and not a new agent.

Everything is measured on one machine — **M2 Max, 32 GB unified memory, 30 GPU cores** —
and every number is committed with the machine it came from. Nothing here is a benchmark
claim about Apple Silicon in general.

- [`docs/VISION.md`](docs/VISION.md) — what this is for, and what it is not.
- [`docs/TECH.md`](docs/TECH.md) — **the answers**, as built and as measured.
- [`docs/features/`](docs/features/) — how each answer was arrived at. All 18 have shipped
  or been dropped; the docs are frozen history.
- [`docs/data/`](docs/data/) — the raw rows every table rests on.

## What was settled

| question | answer | where the numbers are |
|---|---|---|
| runtime | **llama.cpp**, not MLX | [llama.cpp against MLX](docs/TECH.md#llamacpp-against-mlx) |
| model and quant | Qwen3.8-27B `Q4_K_M` | [the measured envelope](docs/TECH.md#the-measured-envelope) |
| KV cache | `q8_0` for K and V | same |
| context | **32k** to grind, **49k** for the editor | [serving](docs/TECH.md#serving) |
| thinking | **off** — 4× the wall clock bought nothing | [sampling and thinking](docs/TECH.md#sampling-and-thinking-settled) |
| sampling | `temp 0.7 / top_p 0.80 / top_k 20 / presence_penalty 1.5` | same |
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
make check                              # the offline gate: gofmt, vet, lint, race tests
make serve CONFIG=config/tuned.env      # llama-server on 127.0.0.1:8081, ~17 GB of weights
make smoke                              # in another shell: does it answer, and call a tool?
make eval N=3 LABEL=tuned-32k           # score the tier-1 suite and print the summary
make help                               # every target
```

The first `make serve` downloads ~17 GB into llama.cpp's own Hugging Face cache. `make
eval` refuses to start when the machine has less headroom than
[`config/machine.json`](config/machine.json) requires — a sweep that pages measures the
pager, not the model.

## How the repo is laid out

```
cmd/        the six commands, each with its exit codes documented in its package comment
internal/   the packages behind them: eval (scoring), harness (adapters), prefix, handoff
config/     one .env per serving config — the single source of truth for every measurement
scripts/    the shell around a measurement: serve, ladder, screen, pair, probe
tasks/      the fixtures the model is scored on; tasks/depth/ is the deep floor check
harness/    how each coding agent is pointed at the local endpoint
mlx/        the MLX runtime that lost to llama.cpp, kept reproducible
mtplx/      the MTP runtime that ran out of memory here, kept for the same reason
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

## The rules the measurements follow

These are not style. Each one has already cost this repo a wrong number, and
[`docs/TECH.md`](docs/TECH.md#gotchas) records what it cost.

- **A run that swapped is void, not slow.** Every row records free memory and the swap
  delta; the reporter names contaminated runs instead of averaging them in.
- **A row says what the *server* reported serving**, never the label a human typed. A
  restart that did not take would otherwise attribute one config's numbers to another.
- **Spread is min–max over three passes**, never a standard deviation — three samples do
  not support one.
- **One toggle at a time, and never one that moves two things.** Thinking carries its own
  sampling, so sweeping the toggle at a fixed temperature measures the pair. `cmd/eval`
  refuses it; that mistake voided 114 rows before it did.
- **A derived number is a hypothesis** and is labelled as one until a run confirms it.
  Arithmetic about this machine has been wrong three times.
- **Free memory is not a pressure signal on macOS.** It sits near zero idle or paging.

## Contributing

[`AGENTS.md`](AGENTS.md) is the work protocol — feature docs, one branch per feature, and
what may be written where. Two rules matter before anything else: a doc marked
`status: Shipped` is frozen history and is never edited, and no change reaches `main`
except through a pull request.

It is written by `kit`, a separate private tool that also runs the board (`kit next`) and
the drift check (`kit audit`). **Nothing in this repo needs it** — `make check` is the gate,
and CI runs that alone — but the protocol's commands will not resolve without it.
