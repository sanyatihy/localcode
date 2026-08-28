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
localcode                          # a session here, with you at the keyboard
localcode "fix the failing test"   # one instruction, run until it is done
localcode status                   # what is being served, at what context
localcode stop                     # stop it, waiting for the memory back
```

Nothing is written to the repository you work in. Session state and handoffs live under
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
