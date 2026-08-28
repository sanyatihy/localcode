# TECH — as built

Durable facts about what this repo actually does, moved here as features ship.
Design arguments stay in the feature docs; this is the state of the machine.

## What is in here

**How it is served and checked**

- [Serving](#serving)
- [Dependencies](#dependencies)
- [Checks](#checks)

**What this machine can carry**

- [The measured envelope](#the-measured-envelope)
- [A conversation is ingested once, and traffic beside it changes nothing](#a-conversation-is-ingested-once-and-traffic-beside-it-changes-nothing)

**How anything here is measured**

- [The harness](#the-harness)
- [The suite is bounded on purpose](#the-suite-is-bounded-on-purpose)
- [Which tier-1 tasks carry signal](#which-tier-1-tasks-carry-signal)

**What the measurements settled**

- [Sampling and thinking, settled](#sampling-and-thinking-settled)
- [Reasoning effort is a four-point axis, and its default is the bad end](#reasoning-effort-is-a-four-point-axis-and-its-default-is-the-bad-end)
- [Tool-call adherence is not a formatting problem](#tool-call-adherence-is-not-a-formatting-problem)
- [llama.cpp against MLX](#llamacpp-against-mlx)
- [Speculative decoding: adoptable at the top of the context, not the bottom](#speculative-decoding-adoptable-at-the-top-of-the-context-not-the-bottom)
- [Nothing displaces Claude Code, and the two axes disagree](#nothing-displaces-claude-code-and-the-two-axes-disagree)

**Driving it**

- [Claude Code against the local endpoint](#claude-code-against-the-local-endpoint)
- [Sessions hand off instead of compacting](#sessions-hand-off-instead-of-compacting)
- [Pi drives a chain for half the ingest, and the incumbent is displaced at this job](#pi-drives-a-chain-for-half-the-ingest-and-the-incumbent-is-displaced-at-this-job)
- [A chain's clock is its model calls, and half its ingest is preamble](#a-chains-clock-is-its-model-calls-and-half-its-ingest-is-preamble)

**Splitting the work across two tiers**

- [The two-tier split, measured](#the-two-tier-split-measured)
- [What the local tier finishes unattended](#what-the-local-tier-finishes-unattended)

**Traps**

- [Gotchas](#gotchas)

## Serving

`scripts/serve.sh <config>` starts `llama-server` from a config file and adds no
flags of its own. `make serve` runs it against `config/tuned.env`, the config the
measurements settled on; `make serve CONFIG=config/<variant>.env` runs a variant. A
variant is a new file, never an edit to an existing one — an edited config silently
changes what an already-recorded number was measured on.

**A ladder rung is not a config file.** `scripts/rungs.sh` derives the contexts worth
walking from what the machine reports, and `scripts/ladder.sh` generates each cell from
`config/tuned.env` with the context and KV type moved. Rungs written by hand were facts
about one laptop, which is the defect the vision names first.

**A config may also name request-level defaults**, which most do not: `TEMP`, `TOP_P`,
`TOP_K`, `PRESENCE_PENALTY`, `CHAT_TEMPLATE_KWARGS` and `CHAT_TEMPLATE_FILE` become
flags only when set. They exist for clients that build their own request bodies. The
scorer sends sampling and the thinking toggle per request and must be served by a
config that sets none of them, or the run measures something the row does not say.

**A config may also size the prefill batch.** `BATCH_SIZE` and `UBATCH_SIZE` become
`--batch-size` and `--ubatch-size` only when set. Every committed config leaves both
unset, and that is a decision rather than an omission: see
[the prefill batch](#the-prefill-batch-was-swept-and-the-default-kept). The defaults are
2048 logical and 512 physical.

**The endpoint is `127.0.0.1:8081`.** Not 8080: that is the port everything else on a
development machine takes first.

### Baseline, as measured 2026-08-17

| | |
|---|---|
| Model | `bartowski/Qwen3.8-27B-GGUF:Q4_K_M` |
| Resolved snapshot | `f0eec4a4bb4975114a030d048952d83c0a53c034` |
| Context | 32768 |
| KV cache | `q8_0` for K and V |
| Flash attention | `auto` |
| GPU layers | 999 (all) |
| Parallel slots | 1 |
| Chat template | `--jinja` (the model's own) |
| Load time | ~4.2 s, process cold and page cache warm |
| Resident memory | ~18.8 GB |

Load time is with the weights already in the OS page cache. A genuinely cold boot
reads ~17 GB off disk first and will be substantially slower.

Resident memory is measured at **low context occupancy** and is not a ceiling — KV
is not fully allocated until used. Finding the real envelope is
[0003](features/0003-establish-the-memory-and-context-ceiling.md).

### Model provenance

Weights live in llama.cpp's Hugging Face cache (`~/.cache/huggingface`), fetched by
`-hf`. The repo keeps no copy: llama.cpp already owns that cache and duplicating it
costs 17 GB for nothing.

`-hf` cannot pin a revision, so the snapshot hash above is the record. If upstream
re-uploads, `-hf` will resolve to something new and the recorded hash is what makes
that detectable. There is no automatic check for this.

## Dependencies

`llama.cpp` from Homebrew, build **10450**. It must support the `qwen35`
architecture, which is what `Qwen/Qwen3.8-27B` reports
(`Qwen3_5ForConditionalGeneration`).

**Re-check after every `brew upgrade`** — this is the dependency that breaks the
model silently rather than loudly:

```sh
strings /opt/homebrew/lib/libllama.dylib | grep -c '^qwen35$'   # expect 1
```

That build also serves the **Anthropic Messages API** at `/v1/messages` and
`/v1/messages/count_tokens`, converting to chat-completions internally, and both
endpoints matter: the second is how a client counts a conversation against the served
tokeniser instead of guessing. Claude Code points at this server with
`ANTHROPIC_BASE_URL` and no proxy, though not with the model's own chat template — see
[Claude Code against the local endpoint](#claude-code-against-the-local-endpoint).

## Checks

`make check` is the offline gate and is what CI runs: `gofmt` cleanliness, `go vet`,
`golangci-lint`, `shellcheck`, a documentation link check and `go test -race`. It needs no
server and no model weights, because CI has neither.

**It covers the repo and not just the Go**, which is half of it by line count. A shell bug
here does not crash — it produces a wrong measurement, and two of the [gotchas](#gotchas)
below are shell bugs that already cost wrong numbers. So `shellcheck` runs over every tracked
script, `scripts/doclinks.py` checks every relative link and heading anchor in the markdown
without fetching anything, and the gate holds shell options to the file mode: **an executable
script must `set -euo pipefail`, and a sourced one must set nothing**, since options set in a
library leak into whatever sourced it.

**`shellcheck` and `golangci-lint` are skipped loudly when absent** and installed in CI, so
the gate is absolute where it has to be. The skip is never total: `bash -n` runs in
`shellcheck`'s place.

`make smoke` runs `scripts/smoke.sh` against a *running* endpoint: health, a chat
completion that must return non-empty content, and a tool call that must return valid
JSON containing the schema's required field. It disables thinking for speed and
determinism — whether thinking helps is
[0005](features/0005-tune-sampling-and-tool-call-adherence.md)'s axis, not a gate's.

`make verify` is both, and is the gate to run before shipping anything that touches
serving. The split exists because the two answer different questions: `check` asks
whether the code holds, `smoke` asks whether this machine is actually serving.

## The measured envelope

Everything in this section is observed, not derived. It is scoped to **Qwen3.8-27B at
Q4_K_M, contexts 8k–64k, on a 32 GB M2 Max**, and re-measuring is required before any of
it is quoted for another model, quant, context or machine.

**Every limit below that code acts on lives in `config/machine.json`** — the headroom a sweep
needs, and each desk profile's context ceiling. Nothing in Go or in a script carries one, and
the ladder's rungs are derived by `scripts/rungs.sh` rather than written down. Another machine
is one file, which is what makes the numbers here a measurement rather than a constant.

### Context costs time, not memory — in this envelope

| ctx | KV | peak RSS | cold ingest | prompt tok/s | swap Δ |
|---|---|---|---|---|---|
| 8 192 | q8_0 | 17.91 GB | 68 s | 109.4 | 0.0 MB |
| 16 384 | q8_0 | 18.64 GB | 149 s | 99.7 | 0.0 MB |
| 16 384 | f16 | 19.07 GB | 139 s | 106.1 | 0.0 MB |
| 32 768 | q4_0 | 18.66 GB | 325 s | 91.0 | 0.0 MB |
| 32 768 | q8_0 | 19.13 GB | 326 s | 90.7 | 0.0 MB |
| 49 152 | q8_0 | 19.72 GB | 543 s | 81.5 | 0.0 MB |
| 65 536 | q8_0 | 20.27 GB | 788 s | 75.0 | 0.0 MB |

Nothing swapped at any rung, so **no ceiling exists that swap or resident size can see**.
What grows is ingest: the prompt rate decays with depth, making a cold 32k context cost
5.4 minutes and a cold 64k cost 13.1.

**A ceiling does exist, and this table cannot show it.** The ladder scored 64k `ok` because
its pass criterion was whether the model completed, not whether the machine stayed usable.
Re-walked against the desktop instead, with WindowServer sampled through each fill:

| context | model | desktop | wired peak | WindowServer |
|---|---|---|---|---|
| 32 768 | ok | **pass** | 21.75 GB | 0.14–0.42 cores |
| 40 960 | ok | **pass** | 21.96 GB | 0.16–0.29 |
| 49 152 | ok | **pass** | 22.21 GB | 0.16–0.33 |
| 57 344 | ok | **pass** | 22.18 GB | 0.15–0.32 |
| 65 536 | ok | **fail** | 22.29 GB | 0.01–0.09 |

The desktop verdict is fixed in advance rather than read off each run. `fail_saturated` is
a sustained ≥ 0.90 cores over any 30 s window and `fail_stalled` is ≤ 0.02; `pass` is
neither, over a run of at least 30 s. `not_applicable` is what an unattended run gets,
since nothing was there to lose. Both bounds sit clear of normal operation: an attended
baseline with no model loaded measures 0.17–0.47 cores.

**The two ranges are different, and both are real.** The model serves 8k–64k. A machine
someone is using is admissible to **56k**; 64k is unattended-only. Every cell above
completed its request, so nothing but the desktop column distinguishes the last row.

The compositor **stalls** rather than saturates — the failing cell's peak sits below every
passing cell's minimum, so the populations do not overlap — and it is flat from the first
sample of the cell, which points at allocation time rather than at ingest.

**That ceiling is conditional on what else is running.** The arithmetic here was wrong for a
session and is restated: the figures below are **anonymous memory plus wired**, which is what
competes for the 32 GB. The earlier version added a per-process RSS sum to the model's RSS,
which counts every shared page once per resident process and counts the model twice — once as
resident, again as wired.

**The model lives in wired memory**, because Metal wires its buffers: with it loaded and
serving, wired measures **20.89 GB** while the mmap'd GGUF holds only 1.99 GB of file-backed
pages. Apps live in anonymous memory — 6.61 GB with an editor open and no browser.

| what | measured |
|---|---|
| model, wired, serving at 32k | 20.89 GB |
| apps, anonymous, editor only | 6.61 GB |
| **total against 32 GB** | **27.50 GB** |

So roughly **11 GB is left for everything that is not the model**, and an editor takes 6.6 of
it. A browser does not fit in the rest: closing one moved the summed-RSS figure by 14.8 GB, and
even discounted for the overcount it is several GB of anonymous memory. **The conclusion stands
— the desk has to be cleared — but it is marginal rather than the comfortable 6 GB overshoot
first reported.** Context is still the wrong lever: an eightfold cut moves the model about
2.2 GB.

The exact margin with a browser open is **not currently measured**, since every figure taken
before 2026-08-19 used the summed-RSS metric. `scripts/memprobe.sh` and `scripts/ladder.sh`
now record `anonymous_gb`, so the next run of either produces the honest number.

**The limit was never read, and it is lower than either guess.** Metal reports a recommended
maximum working set of 22,906,503,168 bytes — **21.33 GiB, exactly two thirds of the 32** —
where the instrument assumed three quarters. So every `wired_headroom_gb` recorded before
2026-08-25 is **2.67 GiB too generous**, and the failing cell's reported 1.71 GB of room was
never there. What is still not established is the mechanism: `vm_stat`'s wired count is all
of the machine's wired memory and the cap governs the GPU's share of it, so the two are not
the same quantity and the 0.54 GB that wired moves across a doubling of context is not what
crossed the ceiling. What a raise experiment would now settle is whether the cap binds at
all — `sudo sysctl iogpu.wired_limit_mb=N` overrides the derivation, and the sysctl is what
this reads first.

**This does not mean memory never binds.** Only one quant was tested. Q6_K weights are
roughly 6 GB heavier, and larger models and longer contexts are untested. 0004's quant
sweep is where memory gets its next real chance.

Marginal KV cost measures **31–37 KB/token** above the first rung, against ~139 KB/token
derived from the config. Some KV allocation is evidently not attributed to process RSS on
Apple Silicon; treat RSS as a lower bound.

### Other measured costs

| | |
|---|---|
| Multimodal projector | **1.02 GB** — 18.26 GB with, 17.23 GB without (`--no-mmproj`) |
| Model load | 4.2 s with a warm page cache; a cold boot reads ~17 GB off disk first |
| Prompt cache reuse | 11 552 of 12 068 tokens reused; a repeated request fell 7.0 s → 2.6 s |
| Smoke gate | 6.5 s |
| Thinking at `xhigh` | 3.06× completion tokens, 1.67× wall, on tier-1 tasks — **this is xhigh vs off, not thinking vs off**; see the reasoning-effort section |

### Ladder rungs are derived, not written down

`scripts/rungs.sh` computes which contexts to ladder over from the machine: total memory,
the weights file's actual size, a reserve for the desktop, the measured KB/token, and an
ingest-time budget. It reports **which of memory or time binds**, which is the ladder's
whole question.

On this machine it derives 8k/16k/32k/64k — the same rungs that were first written by
hand — and reports ingest time as the binding constraint. Modelling 128 GB with `TOTAL_GB=128`
gives **the same rungs**, and so does a 70 GB model: memory allows ~262k tokens in every case
while a 20-minute ingest budget allows ~64k. That is part of why more memory was not bought:
it would not have bought context.

That is worth stating plainly: **more RAM does not buy more context for this model.** It
buys larger quants and larger models. Context is bounded by ingest time, and ingest time
does not care how much memory is spare.

## The prefill batch was swept, and the default kept

`--ubatch-size` is the physical batch: it sizes the compute buffer and the Metal dispatch,
and it governs the half of a turn that prefill owns. It had never been set. Both profiles
were swept upward from llama.cpp's default of 512, and **neither config changed.**

**Where the range stops is the allocator, not a number anybody chose.** The walk doubles
until the machine refuses, and both profiles refuse the same way — the Metal command buffer
fails `kIOGPUCommandBufferCallbackErrorOutOfMemory` on the first prefill batch, at a wired
peak of 22.27 GB. The editor profile reaches that peak one doubling earlier, because its KV
reservation is larger. The server answers `/health` with 200 either way, so the screen that
decides is whether the config can generate a token at all.

| profile | context | admissible | refuses at |
|---|---|---|---|
| `config/tuned.env` | 32 768 | 512–4096 | 8192 |
| `config/agent.env` | 49 152 | 512–2048 | 4096 |

**A batch size does not change what the model answers.** Fixed greedy prompts hash
identically at every admissible cell on both profiles, each against a control that is the
same cell loaded twice. This gated the depth sweep rather than accompanying it: the batch
size changes how a prefill is split, floating-point reductions are not order-independent,
and a batch size that moved the answer would make every number recorded at 512 describe a
different model. The probes have to be long enough to be split differently — a probe that
fits in one physical batch is dispatched identically at every cell and can only report a
match.

**Cold ingest at depth barely moves, and on the profile that motivated the sweep it does
not move at all.**

| profile | 512 | 1024 | 2048 | 4096 |
|---|---|---|---|---|
| grind, 29 491 tokens cold | 352 s | 341 s | 342 s | 343 s |
| editor, 44 236 tokens cold | 573 s | 570 s | 570 s | inadmissible |

The editor profile spans **0.5%** across its whole admissible range. That is the profile
whose 36,309-token turn spent 434 s on ingest, and this axis returns about two seconds of it.

The grind profile moves once, 512 → 1024, and then stops: **3%**, or 11 s off a 352 s fill.
**It was measured once.** No cell has a repeat, so the run-to-run spread is unmeasured; what
bounds it is that 1024, 2048 and 4096 agree within 2 s across three independent cold loads.
Read the 512 cells against that cluster rather than against a repeat they do not have.

**What moving would cost in wired memory.** 1024 costs +0.17 GB over the default. 4096 costs
+0.85 GB and returns nothing over 1024, peaking at 22.12 GB against the 22.29 GB where 0014
lost the desktop — so the top of the admissible range is ruled out on cost wherever it is
admissible at all.

**What would reverse this:** a second pass reproducing 341 s at 1024 against 352 s at 512 on
the grind profile. 3% for 0.17 GB is worth taking once it is a measurement rather than a
single observation. Every cell here was screened `unattended`, so the desktop verdict at
these batch sizes is untaken — a batch size adopted later needs that verdict before it
serves a machine somebody is using.

## A conversation is ingested once, and traffic beside it changes nothing

Measured two ways on `config/agent.env`: a scripted conversation whose every prompt is known
to the token, and two real `claude -p` sessions accounted from the server's own log.

| | prompt tokens | ingested | reused |
|---|---|---|---|
| scripted, 5 turns to 28,126 | 90,400 | 28,121 | 68.9% |
| the same, with a 5,531-token call between every turn | 90,400 | **28,121** | 68.9% |
| the same, that call opening with the conversation's system prompt | 90,400 | **28,121** | 68.9% |
| scripted, 8 turns to 43,195 | 204,916 | 43,188 | 78.9% |
| two real sessions, 22 requests, deepest 21,813 | 286,148 | 34,386 | 88.0% |

**A conversation ingests each token once.** 28,121 is the final prompt of that 28,126-token
conversation, and the per-turn figures are 8,034, then 5,027, then 5,020 a turn — what each
turn adds. Interleaving calls moves none of them.

**The server's host-RAM prompt cache is why.** llama-server keeps a prefix it evicts from a
slot and restores it for the next request that wants it, bounded by `--cache-ram`: 8192 MiB
by default, and `CACHE_RAM` in a config is what moves it — `0` turns the cache off. This model's q8_0 KV costs **138.1 KiB a token** — 65
layers, 4 KV heads, 256 wide for K and V — so that budget holds ~60,700 tokens, more than
this config's whole 49,152 window and a call beside it. **It is bought from the same 32 GB
the weights and the KV reservation sit in**, which 0014's ceiling was walked without.

**But it is a ceiling filled lazily, not a reservation taken at load.** The same config
screened at 49,152 with the cache at its 8192 MiB default and at `CACHE_RAM=0` peaks at
22.386 and 22.398 GB of wired memory — **12 MB apart, in favour of the default**. So the
8 GiB costs nothing a load or a first prompt can see, and what it competes with the KV cache
for is only whatever prefix it has actually saved.

**`selected slot by LRU` does not mean a lost prefix.** All 15 requests of one run logged it
and reused 161,735 tokens between them. It says how a slot was chosen, not what the server
still held.

**A chain gets none of that reuse across a handoff.** Four sessions of one instruction on
`config/driver-mtp-32k.env`, 38 requests, accounted from the server's own log:

| | prompt tokens | ingested | reused |
|---|---|---|---|
| the whole chain, 4 sessions | 269,130 | 31,179 | 88.4% |
| its four session-opening requests | 17,758 | 17,754 | **0.02%** |
| the same instruction finished in one session, 13 requests | 123,973 | 10,039 | 91.9% |

Each session's first request ingests its whole preamble: 4,199, then 4,589, 4,509 and 4,457.
The three that follow a handoff are **43.5% of everything the chain ingested** and 139.3 s of
its 1,147 s — **12.1% of the wall**, spent on tokens the server had held minutes earlier.

**It is not slot rejection.** All three were `selected slot by LCP similarity` at
`f_sim_best` 0.638–0.657 against a 0.100 threshold, so the slot holding the last session's
conversation was chosen every time. The slot was chosen and the common prefix was still one
token, which places the divergence at the **head** of the preamble rather than its tail: what
is lost is not the tokens after a handoff path is named, it is all of them.

**The two paths that made it differ are the chain's now, not the session's.** The system
prompt carried the session number twice — `--add-dir` and the handoff path the briefing
names — and nothing else about it moved: three sessions' prompts, captured at the endpoint,
differ in those two strings and in nothing else. Both now name the chain's directory, and
the handoff is moved into the session's own as soon as it has been read, so a chain's
sessions send byte-identical preambles while every row downstream still finds one handoff
per session.

**That is worth a third of everything a chain ingests.** The same instruction, the same
config and the same ceiling, before and after, both finishing 20/20 in four sessions:

| | ingested | reused | the three requests after a handoff | their prompt time |
|---|---|---|---|---|
| the session number in the prompt | 31,179 | 88.4% | 13,555 ingested, 0.0% reused | 139.3 s |
| the chain's paths instead | **20,941** | **92.2%** | **2,236 ingested, 80–85% reused** | **24.6 s** |

**A handoff now costs 745 tokens, not 4,518.** Each session-opening request reuses 3,664–3,665
tokens — the same figure to a token across all three, which is the shared preamble and the
evidence that it is shared. What is still ingested is what genuinely differs: the handoff the
session inherits, which is a different plan every time.

**The wall barely moved — 1,147 s to 1,132 s — and that is not the measurement.** The second
chain generated 8,652 tokens against 7,924 doing the same twenty bugs its own way, so the
clock carries work that differs; the token columns do not.

**Only a prefix the server has never seen ingests from zero**, and across two whole sessions
that is one request. A fresh session is not one: the second session's first request sent the
same 3,130-token preamble and ingested 516 of it, 4.5 s against 27.6 s. So about **516
tokens of Claude Code's preamble differ between two otherwise identical runs**, at its tail.

**What traffic beside the conversation costs is its own ingest** — 215.9 s over four calls at
28k, 375.9 s over seven at 43k. That is charged to the session's wall clock, and no slot
count or second endpoint removes it.

### Reading it yourself

`cmd/prefixprobe` replays a fixed conversation and records what each turn was charged;
`scripts/prefixrun.sh` walks one config through the conditions, restarting the server between
them so the second is not served the first's leftovers. `cmd/prefixlog` does the same for
traffic nobody scripted, off the server's log. Its one derived figure — the prompt, which the
server does not print — is checked against the endpoint's own rows with `-check`, and agrees
on every request and every field of the ceiling run.

## The harness

`cmd/eval` drives a fixed suite against a running server and writes one JSON row per run,
into `docs/data/`.

- **Client-measured wall time is the only speed metric that crosses backends.** It is
  recorded on every row, including a run that failed or exceeded its budget. Server-reported
  `gen tok/s` and `prompt tok/s` come from llama.cpp and may not exist elsewhere, so they are
  recorded where available and never used to compare one runtime against another.
- **Every row records free memory and the swap delta across the run**, and the reporter names
  runs that swapped instead of averaging them into the timings — a run that swapped measured
  the pager. Rows that could not measure are reported as unverified rather than as clean;
  `mem_measured: false` is not the same as a swap delta of zero.
- **A backend that cannot introspect is still scoreable.** llama.cpp exposes `/props`; a
  backend that does not is run anyway, with the served config recorded as unavailable and the
  guard that checks it against the typed label switched off and said so.
- **Model-specific behaviour lives in a profile, not in the scorer** — `config/profiles/`.
  The thinking mechanism, each mode's sampling pair, and where reasoning arrives are all
  properties of the model, and all three are read: a profile naming a mechanism the scorer
  cannot perform is refused at load rather than leaving the toggle unset. The pair is there so
  a toggle cannot be swept at one fixed temperature by accident, which has cost 114 rows.
- **A row records what the server reported serving** — `n_ctx`, model file, and the
  `reasoning_effort` sent — not the label a human typed. A label is a claim; a restart that
  did not take would otherwise attribute one config's numbers to another.
- **Spread is min–max over three passes, never a standard deviation**, which would claim
  precision three samples do not have. Runs are sequential: the server has one slot.
- **A tier-1 task must have exactly one defensible action.** A task that scores a style
  preference — reading a file before editing it — fails every config identically and ranks
  nothing.
- **The suite is split by what a task can detect.** `tasks/` ranks — nine patch and
  tool-call fixtures, ~95 s a pass. `tasks/depth/` floor-checks recall at 2k–16k and is run
  only when the KV cache type, the backend or the model changes, because that is what could
  damage it. It was 71% of a pass's runtime (227 s of 321 s) while returning 3/3 at every
  setting ever measured, which is most of the clock for no ranking.
- **A fixture is proved against answers that are merely different, not just against wrong
  ones.** `patch-off-by-one` failed a correct fix that returned `nil` rather than `[]int{}`
  for the empty cases — a distinction `reflect.DeepEqual` draws and the spec does not. That
  is the same defect as scoring a style preference, and it costs a config marks for being
  right, so the fixture self-tests now assert that equally-defensible answers pass.
- **Patch fixtures are proved to discriminate before any model time is spent on them.**
  `TestPatchFixturesDiscriminate` runs a correct answer and the tempting wrong one through
  the real patch runner and requires the first to pass and the second to fail. Both halves
  are asserted: a fixture whose unseen test rejects a correct fix scores the model down for
  being right, which is the more expensive of the two failure modes.
- **A retrieval answer naming a decoy fails even when the wanted key is also present**, so
  reciting every key in the dump is a failure to discriminate rather than a hedge that earns
  a pass. The three pre-distractor retrieval prompts are pinned by hash: their numbers are
  already recorded in `docs/data` and a moved prompt would break comparability silently.
- **Tier 2 refuses a fixture that passes before the harness runs**, and refuses it without
  calling the driver.
- **Harness configuration is repo-local and different for each**: Pi an extension
  registering a provider, OpenCode a `provider` block using `@ai-sdk/openai-compatible`,
  Hermes a top-level `model:` block with `provider: custom`.
- **Pi's provider file reads the served context from `/props`** rather than declaring a
  number, because no literal is right for both the comparison's 65,536 and the agent flow's
  49,152. With no server to read, it refuses to load and names the config to serve.
- **Hermes refuses any context window under 64,000 tokens**, checked before any request. Pi
  and OpenCode run at 32k and Hermes cannot, so a like-for-like comparison must put all three
  at 64k — where a cold ingest costs 13.1 minutes against 5.4 at 32k.

## The suite is bounded on purpose

Every task carries `timeout_seconds` and over-budget is scored as `fail_over_budget`,
separately from any quality outcome. The default is 120 s; the fixtures that legitimately
cost more say so, up to 300 s for the 16k retrieval. No run has yet hit one on real work — the guard is covered by a unit test, not by a live
firing, and the `xhigh` cell that used to run unbounded now finishes at 95 s against its 120 s
budget. This is not a safety net that should ever fire — it is what stops a sweep being open-ended, after a single task spent 15 minutes
reasoning and produced no answer.

**The grader is denied the network.** Both tiers score an answer by running `go test` over
code the model wrote, so it runs with `GOPROXY=off` — an invented import fails against the
module cache rather than being fetched, and is scored as invalid Go — and, where
`sandbox-exec` is, under a generated profile allowing nothing but loopback.

Budgets are set from the fast end (`reasoning off`) plus headroom, so a task hitting its
budget means something changed, not that the number was tight. One pass over the 14 tasks
costs **5.4 minutes** with reasoning off; the same pass at `xhigh` cost 16.8 and did not
finish six of its runs.

**`reasoning_effort` is passed to the server verbatim rather than checked against a list.**
Qwen3.8 takes `low`/`medium`/`xhigh` and the next model will take something else; a harness
that hardcodes one vendor's vocabulary has to be edited before it can measure anything new.
What is guaranteed instead is that whatever was sent appears on every row.

## Which tier-1 tasks carry signal

Measured at `off` and `xhigh` across the full 14-task suite, and at all four levels for the
three below that moved.

| tasks | verdict |
|---|---|
| `patch-contradiction-rounding` | **discriminates** — the only task with a genuine, repeatable split |
| `toolcall-constraint-readonly` | **weakly discriminates, in the opposite direction** — off 2/3, every reasoning level 3/3. Its earlier failures were a fixture flaw (the model refusing to patch a file it had not been shown), fixed by supplying the source in the prompt and re-measured |
| `patch-nil-check`, `patch-off-by-one`, `patch-sibling-merge`, `patch-sibling-splitpath`, `retrieval-2000/8000/16000`, `retrieval-distractor-2000/8000`, `toolcall-constraint-unknown-path`, `toolcall-edit-file`, `toolcall-read-file` | **flat** — no quality failure at any setting measured; `patch-off-by-one` non-terminated once at `xhigh`, which is a budget outcome and not a wrong answer. They are the floor check that catches a config broken outright, and they cost seconds; they cannot rank anything |

A summary over the whole suite is therefore diluted by twelve columns that cannot move. Read
the discriminating subset, and keep the rest as the floor check they are.

## Sampling and thinking, settled

Each mode at its own model-card sampling; 9 ranking tasks, 3 passes, 32k/q8_0.

| setting | pass | tool-call valid | wall |
|---|---|---|---|
| **off · `temp 0.7 / top_p 0.80 / top_k 20 / presence_penalty 1.5`** | **25/27** | 12/12 | **5.0 min** |
| off · greedy | 21/27 | 12/12 | 4.4 min |
| off · temp 0.3 | 23/27 | 12/12 | 4.3 min |
| off · temp 1.0 | 24/27 | 12/12 | 8.4 min |
| off · top_p 0.95 | 25/27 | 12/12 | 4.4 min |
| **on/low · `temp 1.0 / top_p 0.95 / top_k 20`** | **25/27** | **12/12** | 18.6 min |
| on/low · greedy | 24/27 | 12/12 | 16.4 min |
| on/low · temp 0.7 | 24/27 | 12/12 | 18.0 min |
| on/medium | 24/27 | 12/12 | 22.4 min |

**Nothing beats the model card, in either mode.** Colder is monotonically worse without
thinking — 21, 23, 25 as temperature rises to 0.7 — and `top_p` does nothing at all. **Greedy
is the worst setting measured**, which is worth knowing because it is the tempting choice for
a reproducible sweep.

**Tool-call validity is 12/12 in every cell of the sweep and ranks nothing.** It is the column
0005 was built around, and no setting moves it: every call this model emitted was parseable and
schema-conforming. Greedy's deficit is reasoning, not format. Where a tool-call task fails it
is a *choice* — the right shape addressed to the wrong tool — which is the same conclusion the
150-run classification reaches from the other direction.

**Reasoning does not earn its cost at either profile.** It is 4× the wall clock for 25/27
against 25/27, with tool-call validity identical at 12/12. Nothing measured here favours it.
Eight of nine tasks are 3/3 in every cell, so the claim is *no gain detectable on this suite*,
not *no gain exists*.

So both profiles take the same setting, which is a result and not an assumption:

| profile | thinking | sampling |
|---|---|---|
| attended | **off** | `temp 0.7 / top_p 0.80 / top_k 20 / presence_penalty 1.5` |
| unattended | **off** | as above — nothing was found for the 4× to buy |

The modes fail *differently* on the one task that moves, which the pass rate hides: off fails
the plain-arithmetic cases both readings of a contradictory spec agree on, while thinking gets
the arithmetic right and then resolves the contradiction case by case. Any later claim that a
mode is better must say at what.

## Reasoning effort is a four-point axis, and its default is the bad end

Qwen3.8 ships a `reasoning_effort` dial — `xhigh` (default), `medium`, `low` — separate from
`enable_thinking`. `llama-server` honours it both as a top-level field and inside
`chat_template_kwargs`, with identical results; the harness sends the top-level form the model
card documents and records the level on every row.

**It is a system message the template injects, not a sampling parameter** — `low` renders
"Keep your thinking brief and focused", `xhigh` "think carefully… validate key assumptions…
consider plausible alternatives". It is prepended to the task's own system prompt rather than
replacing it, and is absent entirely when `enable_thinking` is false, so the two knobs compose.
With nothing set the template injects the `xhigh` text, which puts the default beyond inference.

**The default is `xhigh`, and it is not a sane default for agentic work.** Qwen's own notes
scope it to "complex tasks demanding thorough analysis". On this suite it does not
terminate: three tier-1 tasks burned their whole budget on reasoning and never wrote an
answer, one producing 27,234 characters of it. Every thinking-mode number recorded before
2026-08-18 was taken at `xhigh` whether it says so or not.

The three levels on the three tasks that move (three passes each, 32k/q8_0). All three share
the toggle and the sampling, so they are comparable with each other; the `off` column that
used to sit beside them was taken at a different sampling and is void, and
[Sampling and thinking](#sampling-and-thinking-settled) is the clean comparison across the
toggle.

| task | low | medium | xhigh |
|---|---|---|---|
| `patch-contradiction-rounding` | 1/3 | 2/3 | 0/3, never terminated |
| `patch-off-by-one` | 3/3 | 3/3 | 2/3, one non-termination |
| `toolcall-constraint-readonly` † | 3/3 | 3/3 | 3/3 |

† re-measured after its fixture was fixed. Its `xhigh` runs complete once the cap is 4096
rather than 1024, so that task's non-termination was a budget problem — unlike the
contradiction task, where raising the cap only bought a longer spiral.

**More reasoning is not a free upgrade, and it does not move in one direction.** On the
contradicted-specification task thinking fails 3 of 6, always on the same assertion, the model
resolving the contradiction case by case rather than picking one rule. On the read-only
tool-choice task the direction reverses: every run with reasoning on takes the constraint.
`medium` beating `low` is within the noise of three runs and is not a ranking. Two tasks and
two directions is enough to kill "more thinking is better" as a working assumption, not enough
to replace it with a rule.

## Tool-call adherence is not a formatting problem

Across **150 recorded tool-call runs**: 136 pass, 10 wrong-but-valid, 2 from a fixture since
fixed, 2 truncated at a cap. **Zero unparseable, zero schema-invalid.** The entire remaining
deficit is choosing the wrong tool under a stated constraint.

Constrained decoding is therefore not pursued: a grammar makes malformed calls impossible and
we have none, while it cannot fix tool choice and would cost sampling speed. The template also
asks for an XML call form — `<function=name>` with `<parameter=key>` — so a JSON-schema
constraint would fight it rather than help.

## llama.cpp against MLX

Same model, matched by footprint — llama.cpp Q4_K_M at 17 GB against MLX 4bit at 16.1 GB —
driven through the same scorer at 0005's settled config, 42 rows each.

| | llama.cpp | MLX |
|---|---|---|
| pass | 40/42 | 39/42 |
| tool-call validity | 12/12 | 12/12 |
| wired, model serving | 20.89 GB | **18.02 GB** |
| minimum free memory | 0.06 GB | **2.64 GB** |
| runs that swapped | 7 | **0** |
| cold depth prompts | **3–9% faster** | |
| short prompts | | **faster** |
| decode, client-side | 8.89 tok/s | **10.35 tok/s** |
| warm reuse, identical request | 20s → 2s (10×) | **19.6s → 0.5s (39×)** |

**They differ in where the KV cache comes from, and that is the whole story.** llama.cpp
reserves its cache at load against `--ctx-size`, so reuse is free within that reservation and
the cost is paid once. `mlx_lm` allocates cache capacity **eagerly per slot at startup**:
`--prompt-cache-size 16` left 0.11 GB free on the first request, where 2 slots left 5.38 GB.
Cache capacity is bought from the same wired pool the weights sit in, so on 32 GB reuse
breadth and depth headroom trade directly against each other.

**Unbounded is not an option.** `mlx_lm`'s LRU is unbounded by default, and one 16k prompt
drove free memory to zero — with swap flat, because wired pages cannot be paged out. Every
later request stalled rather than slowed. `--prompt-cache-bytes` and `--prompt-cache-size`
are mandatory on this hardware, not tuning.

**The decision is to stay on llama.cpp**, and it is closer than the table suggests. MLX wins
on memory, which is the constraint 0014 showed binds here, and on warm reuse. It loses on
three things that matter more today. It serves no Anthropic `/v1/messages`, which the editor
flow needs. It reports no served config and no timings, so a run cannot be checked against its
label. And under memory pressure it stalls hard rather than degrading, which took three
misconfigurations to diagnose.

Decode is measured client-side because `mlx_lm` reports no server-side rate; llama.cpp's own
figure for pure decode is 9.51 tok/s, so MLX's true decode is higher than the 10.35 shown,
which includes prefill. Both sit near 40% of the ~25 tok/s ceiling that 16.1 GB of weights per
token implies at this machine's ~400 GB/s — normal for real kernels, and the reason no
configuration change reaches the figures quoted for speculative decoding.

**Multi-token prediction is reachable, and not through this server.** `mlx_lm` 0.31.3 rejects
`mlx-community/Qwen3.8-27B-MTP-4bit` with `Model type qwen3_5_mtp not supported`. The lever is
on llama.cpp instead: the served GGUF carries its own MTP head, which stock builds drop as
unused and PR #27342 picks up. It is measured at 1.26–1.57× depending on prompt depth, and the
figures are under [Speculative decoding](#speculative-decoding-adoptable-at-the-top-of-the-context-not-the-bottom). MTPLX, the MLX runtime that does implement
native MTP, loads on this machine and then runs out of GPU memory under a real prompt.

**What would reverse it**, now that a larger machine is decided against: `mlx_lm` gaining
`qwen3_5_mtp` support *and* beating llama.cpp's own MTP path, which is a measured number rather
than a hypothetical. Or MLX gaining `/v1/messages` — or the driver no longer needing it, which
is what a chain on Pi would mean. **The memory route is not closed after all**: it was read as
closed because more RAM was what would have stopped slot count competing with the model, and
the ceiling turns out to be a cap at two thirds of the RAM already installed rather than the
RAM itself. `mlx_lm` buys cache capacity from that pool eagerly, so a raised cap is the "more
memory" this paragraph ruled unavailable. **MTPLX has a route, it was taken, and it still
does not fit.** Screened at 24,576 MiB the `Optimized-Speed-FP16` checkpoint answers 200 to a
44,236-token prompt at 49,152, where at the derived cap it errored — but it peaks at 26,089
and 26,302 MiB across two runs and swaps both times, so both rows are `void_swapped` and are
read for the peak rather than for a verdict they do not carry. Admitting it needs a cap above
26,302 MiB, which leaves the system under 6.3 GiB and `scripts/gpuraise.sh` refuses. **What
excludes it here is the reserve, not the absence of a route.** Its peak arrives during the
fill and not the load — 21.0–21.2 GiB wired once loaded, either run — because it allocates KV
per request, so a cap does not bound it the way it bounds llama.cpp.

**One caveat on the benchmark itself.** The suite interleaves 14 distinct prompts before
repeating any, which is what forced the slot-count problem. A real agent session is one
conversation resending a growing prefix, needing one or two slots — the configuration that is
memory-safe. So this comparison understates MLX for the workload the project actually cares
about, and the honest reading is that neither runtime is disqualified.

## Speculative decoding: adoptable at the top of the context, not the bottom

Three candidates were screened against 0014's desktop rule before any suite ran, and two
never generated a token on this machine. What survives is the model's own multi-token
prediction head, which the served GGUF has carried all along. Stock llama.cpp logs those
tensors as unused and drops them; the build from PR #27342 makes an MTP draft context against
the same weights, loading no second model.

| candidate | extra weights | verdict |
|---|---|---|
| **native MTP**, `--spec-type draft-mtp` | none — the target's own head | admissible at 32,768, refused at 49,152 |
| DFlash2 drafter, PR #27342 | 1.1 GB | GPU out of memory at load, both contexts |
| MTPLX (MLX, native MTP) | a 20.68 GB checkpoint of its own | out of memory under a real prompt at the derived cap; at 24,576 MiB it answers, peaking at 26,089–26,302 MiB and swapping |

**The ratio is of decode and not of wall, measured client-side from the gap to the first
token.** A speculative decoder moves decode and cannot move prefill. At depth prefill is most
of the clock: one validation run spent 279 seconds, 247 of them before the first token.
Server-reported rates are not used for the comparison, per [the harness's own rule](#the-harness).

| prompt depth | baseline | native MTP | ratio | acceptance |
|---|---|---|---|---|
| ~200 tokens (the ranking suite) | 0.1050 s/tok | 0.0669 | **1.57×** | 3.94 |
| 8 000 | 0.1181 | 0.0844 | 1.40× | 3.96 |
| 16 000 | 0.1348 | 0.1005 | 1.34× | 3.97 |
| 32 000 | 0.1700 | 0.1346 | **1.26×** | 3.97 |

**Acceptance does not decay; the cost of a verification step does.** Nearly four tokens are
committed per step at every depth. Both sides slow anyway — 0.105 to 0.170 s/token on the
baseline — because speculation cannot make attention over a longer cache cheaper. So the same config is adoptable against short prompts and, by the same rule, is
not against long ones.

**It is lossless by measurement.** Fixed prompts at temperature zero hash identically with
the mechanism on and off. That is what licenses reading the speed number at all: a decoder
that changed the answer would be measuring something else.

**The verdict, per profile.**

- **Grind, unattended, 32,768 served**: adopt. 1.57× on the ranking suite clears the 1.5×
  bar set before the runs, and it costs 0.43 GB of wired memory. Pass rate is 23/27 against
  25/27 on the two discriminating tasks, at sampling 0.7; the identical greedy hash rules that
  out as a distribution change.
- **Long prompts**: record, do not adopt. 1.26× at 32,000 tokens sits inside the band the
  rule reserves for "measured, not taken", and an agent session's prompt is deep.
- **Editor, 49,152**: **admissible above the derived cap, and the cap is what refused it.**
  At the 21,845 MiB Metal derives, the allocator fails on the first prefill batch —
  `kIOGPUCommandBufferCallbackErrorOutOfMemory` at `n_batch = 2048`, 500 within a second, and
  identically at `CACHE_RAM=0`, so the host prompt cache was never why. Raised to 24,576 MiB
  by [`scripts/gpuraise.sh`](../scripts/gpuraise.sh), the same config loads in 6–16 s and
  answers 200 to the same 44,236-token prompt, peaking at 23.269 GiB with 0.731 GiB to spare
  and a swap delta of 0.0. One thing moved, and it was the one nothing had moved.
- **Attended, 49,152 at 24,576 MiB**: **taken, and it passes.** WindowServer sustained
  0.155–0.275 cores against its own 0.114 baseline, swap delta 0.0. So the head is adoptable
  at the editor context on a machine somebody is using — with an editor and no browser, which
  is what a 23.269 GiB peak leaves room for. It cost three runs: one swapped and voided, one
  scored `fail_stalled` on a desktop nobody was driving, and the rule cannot tell that apart
  from a compositor that died.
- **Attended at the derived cap, 32,768**: still undecided, and still not guessed. That config
  peaks at 22.10 GB where the desktop died at 22.29, and no screen has been driven against it.
- **Driver, 32,768 served**: adopt for work whose calls are cheap, and not by default.
  Measured on a whole chain, which is what the trade is between — but the chain was a
  generated fixture, and on real source the ceiling it leaves ends every session. The driver
  serves 49,152; see below.

**A driver chain is faster at the smaller context, having paid the extra session.** The
"record, do not adopt" verdict above rests on an agent session's prompt being deep, and the
driver's is not: its preamble is 4,395 tokens against the 36,309 an editor session's first
request measured, so 49,152 was capacity chosen for a flow this is not. One instruction —
twenty independent Go bugs, one per file — driven to completion on each config, scored by
`go test` and not by what the sessions said:

| | sessions | wall to the answer | ingested | generated | peak context |
|---|---|---|---|---|---|
| `config/agent.env`, 49,152 | 1 | 824 s | 14,234 | 4,311 | 16,294 of a 40,960 window |
| `config/driver-mtp-32k.env`, 32,768 | 2 | **661 s** | 17,119 | 4,108 | 11,611 and 7,341 of 24,576 |

**163 seconds, or 20%, for a second session and 2,885 more ingested tokens.** That is the
arithmetic working out the way it was not obvious it would: three times the handoffs against
a decode ratio near 1.3. Per generated token the chain ran 0.191 s against 0.161 — 1.19×,
below the 1.26× pure decode gives at 32,000 tokens of prompt, because a chain's clock holds
ingest and tool time as well.

**One run a side, no repeat.** At the served sampling the two chains did not do identical
work — 46 tool calls against 35 — so the wall clock is read against the generated tokens,
which agree within 5%. The builds differ too, since the comparator is the shipped config:
`PATH`'s `llama-server` served 49,152 and PR #27342's build served 32,768. Both finished
20/20, and `fixme_test.go` was byte-identical to a freshly generated one in both checkouts
afterwards.

**One hang in 27 runs**, returning no token in 240 seconds against a budget it then hit.
Once is not a characterisation, and it is recorded rather than explained.

**Re-asked at one depth, and the head still pays.** A 25-session chain read 6.93 tok/s with
the draft head against 6.94 without it, which would have inverted the verdict above. It did
not survive being measured: those sessions ran at different context depths, and decode falls
with depth whatever is drafting. The same twenty-bug fixture run a third way settles it —
`config/agent.env` pinned by `-ceiling 25` to the driver's own 10,240 ceiling, so the two
arms differ in the draft head and not in how deep they work:

| | ceiling | sessions | peak context | wall | generated | s / generated token | decode |
|---|---|---|---|---|---|---|---|
| `config/agent.env`, 49,152 | 22,528 | 1 | 18,336 | 850 s | 4,589 | 0.185 | 6.77 tok/s |
| `config/agent.env`, 49,152, `-ceiling 25` | 10,240 | 2 | 12,276 and 6,452 | 782 s | 4,184 | 0.187 | 6.75 tok/s |
| `config/driver-mtp-32k.env`, 32,768 | 10,240 | 2 | 11,935 and 6,800 | **640 s** | 4,491 | **0.143** | **9.33 tok/s** |

**Pinning the depth moves the ratio by nothing: 1.38× either way.** The pinned arm's peaks
match the driver's within 3% session for session and its decode reads 6.75 against the
unpinned 6.77, so depth accounted for none of the gap and the head accounted for all of it.
Against the pinned arm the driver is 142 s, or 18%, on equal sessions — 1.31× per generated
token, which is the same figure from a third direction. The rows are in
[data/2026-08-24-m2max-32gb-0034-draft-head-chain.jsonl](data/2026-08-24-m2max-32gb-0034-draft-head-chain.jsonl).

**Read a decode rate against the depth that produced it, or do not read it.** That is the
lesson the inverted signal leaves: two sessions of one chain are not a controlled pair, and
a rate quoted without the peak context beside it cannot be compared with another one.

**Attended, and one run an arm.** The operator was working on the machine throughout, so
the wall clock carries whatever that contended for; the counts and the within-run decode
ratio do not. All three finished 20/20 by `go test`, with `fixme_test.go` byte-identical to
a freshly generated one afterwards. `result_cap` follows the window rather than the ceiling,
so the pinned arm still returned up to 10,240 bytes of a `Read` against the driver's 6,144 —
pinning the depth does not pin that.

**And on real source the smaller ceiling loses anyway, because the fixture cannot reach
it.** A call on the generated fixture costs 298 tokens against the 683 measured on real
work, so the 5,728 tokens a 10,240 ceiling leaves above the preamble buy about nineteen
calls on the fixture and about eight on a repository. The same 36-session chain 0032's cost
figure comes from ran at both ceilings:

| | ceiling | sessions | median calls | ended on the ceiling | preamble share of ingest | decode | s / call | s / generated token |
|---|---|---|---|---|---|---|---|---|
| 32,768 served | 10,240 | 17 | 9 | **17 of 17** | **46%** | 8.77 tok/s | 36.5 | 0.161 |
| 49,152 served | 22,528 | 18 | 31 | 7 of 18 | 29% | 6.65 tok/s | 37.4 | 0.177 |

**Every session at the smaller ceiling ended on it, and none reached its call budget.** The
1.32× decode advantage survives as 10% per generated token and as nothing per tool call:
what eats it is the preamble, which rises from 29% of everything ingested to 46% because a
handoff falls due every nine calls instead of every thirty-one. `localcode` therefore
defaults to `config/agent.env`, and `-config config/driver-mtp-32k.env` is what moves it.
The rows are in
[data/2026-08-24-m2max-32gb-0034-real-work-ceilings.jsonl](data/2026-08-24-m2max-32gb-0034-real-work-ceilings.jsonl).

**That chain is a signal, not a controlled pair.** It interleaved the two configurations
across phases of one instruction, so the sessions at each ceiling did different work — the
same objection that sank the reading this feature was drafted on. What it is not vulnerable
to is task mix: seventeen of seventeen is a count, and a ceiling that stops every session is
not a timing artefact. The controlled pair above runs a fixture whose calls cost a third of
real ones, so between them the question of a controlled run on real source is open.

**The context ceiling is 38,912, and it is the draft context that sets it.** The MTP path
builds a second `llama_context` over the same weights, so there is no second copy. Its cache
is still sized at the context the target serves, so its cost grows with `--ctx-size`.
Measured: 32,768 and 36,864 serve, 38,912 serves a 35,020-token prompt at **22.28 GB**, and
40,960 refuses on the first prefill batch. That ceiling sits below the editor profile's
49,152 and above the grind profile's 32,768, so the fast config is available to the scorer
and not to the editor. **Every figure in this paragraph is at the derived cap**, which is now
known to be what refuses the head at 49,152; whether a raise moves the 38,912 ceiling with it
is untested, because nothing re-walked it.

`--n-gpu-layers auto` does not move it. Every refusal logs `common_fit_params: failed to fit
params to free device memory: n_gpu_layers already set by user to 999, abort`, so this
project's pinned value was blocking the build's own fitter. Unpinned at 49,152 it reaches
22.255 GB and still refuses. The lever is measured and spent.

A caveat that matters more than the ceiling: **38,912 peaks at 22.28 GB, and 0014's desktop
died at 22.29.** Serving the ceiling and using the machine are not the same question, and
nothing here answers the second.

## Nothing displaces Claude Code, and the two axes disagree

Four harnesses over five patch fixtures, three passes each — 60 runs at one serving config
(`config/harness.env`, 65,536), each from a cold state and bounded at ten minutes. Tokens
and turns are the server's own counters read either side of every run, because each harness
accounts for its work in units of its own.

| harness | passed | turns | ingested/task | context/turn | wall, clean rows |
|---|---|---|---|---|---|
| **Claude Code** (baseline) | 14/15 | 3.9 | 3,514 | 3,336 | 127.5 s (5 of 15) |
| Pi | 12/15 | 3.9 | **735** ×0.21 | 1,857 | 63.6 s (12 of 15) |
| OpenCode | 13/15 | 6.8 | 1,942 ×0.55 | 6,909 | 96.8 s (8 of 15) |
| Hermes | 15/15 | 9.2 | 14,119 ×4.02 | 12,429 | 299.4 s (12 of 15) |

**No challenger wins on both axes, and the order inverts between them**: the cheapest is
the least reliable and the most reliable is the dearest. Hermes' extra task is one run in
fifteen. The bar for switching was a clear win on tokens per completed task without giving
up quality, tokens being what this hardware actually rations; Pi's ×0.21 arrives with two
more failures in fifteen, which is not that. The incumbent stays — the question was whether
anything beats what is already in use, and nothing here does.

**The six failures are two modes, and each tracks one of the axes.** Three sit on
`patch-contradiction-rounding`, where the doc comment and the test that must keep passing
disagree. It caught the two low-turn harnesses — Claude Code 2/3, Pi 1/3 — and neither of the
two that take more turns. Three sit on `patch-sibling-splitpath`, all the same compile
error — an in-place edit that drops the `strings` import — and it caught the two harnesses
that edit (Pi 2/3, OpenCode 1/3) and neither that rewrites the whole file. Frugality costs
verification; editing costs imports. Three runs a cell, so this is a pattern rather than a
rate.

**Preamble size does not explain the token spread.** Pi's fixed preamble is *larger* than
Claude Code's at the same three tools, 3,922 against 3,711, yet Pi ingests a fifth as much per
task. Claude Code re-ingests roughly its whole preamble on every run; Pi's survives in the
server's prefix cache. **What changes in that prefix between two runs is
516 tokens at its tail**, measured since: a second session's first request reused 83.5% of a
3,130-token preamble. A preamble re-ingested in full is therefore a server that has not seen
it, rather than a prefix that differs.

**Every harness completes a task offline**, with the network denied in the kernel and only
the loopback the model is served on left open. Claude Code included: it holds under token
authentication with nonessential traffic off, which is what
[`harness/claude-code/claude-code.env`](../harness/claude-code/claude-code.env) sets. So the
offline axis separates nothing, and the vision's "at least one path is genuinely offline"
outcome is already met by the incumbent. The OAuth refresh path a claude.ai login would use
stays untested, as in 0008.

**Hermes is unattended-only on this machine and did not converge once.** Its 64,000-token
floor sits above 0014's attended ceiling of 57,344 with no overlap, so it is admissible only
when nobody is using the machine. One run before the budget existed spent 45 minutes and 90
turns on a fixture the others finished in three; bounded at ten minutes it then passed all
fifteen. A harness that does not converge is `fail_over_budget`, not a broken adapter.

**Timings are the weakest column here.** At 65,536 with an editor resident the machine pages,
and a run whose swap grew measured the pager — so wall is averaged over clean rows only and
the count of them is printed beside it. Tokens and turns are indifferent to paging.

## Claude Code against the local endpoint

**Both the driver and the editor serve 49,152, and the number that decided it for the
driver is nine.** That is the median tool calls a session got through at the 10,240 ceiling
32,768 leaves, against thirty-one at 22,528 — and every one of those seventeen sessions ended
on the ceiling rather than on its budget. The smaller context is the faster one per token and
the MTP head it admits is a real 1.38×, measured at one depth: [the chain, three
ways](#speculative-decoding-adoptable-at-the-top-of-the-context-not-the-bottom). It is still
the wrong default, because a chain pays for that speed in handoffs and on real source the
handoffs fall due three times as often. `localcode` therefore defaults to
`config/agent.env`, and `-config config/driver-mtp-32k.env` is what moves it — for work whose
calls are cheap, which is what the generated fixture measures and a repository is not.

`config/agent.env` is the serving config for an editor agent. It serves **49,152**
rather than the scorer's 32,768, and differs otherwise in what it serves rather than in
what it loads. The context is capacity, not tuning: an extension session's first request
measures **36,309 tokens** — the full tool set, the project's instructions and the
editor's context — which does not fit 32,768 before anybody types. **A terminal session
answers the same question in 45 s against the extension's 462**, on the same model and server.
`--tools` cuts the preamble to 3,767 tokens, and the extension has no equivalent — so its
preamble is not reducible from the client side.

**Raising the context buys no speed.** Prefill costs what the prompt is, not what the
context reserves. That 36,309-token turn measured **434 s of ingest at 83.7 tok/s**, then
generated 82 tokens at **5.54 tok/s** — 7.5 minutes end to end. Decode decays with depth
too: the same server answers a short prompt at 9.8 tok/s, so depth costs both halves and
not just the prefill. Sampling and the
thinking toggle are per-request for the scorer, and a client that builds its own request
body sends neither — so 0005's settled pair and `enable_thinking: false` are served as
defaults. Without that, this model answers at its `xhigh` default.

**The model's own chat template cannot serve this client.** Claude Code sends a
`role: "system"` message after the user turn — the `mid-conversation-system` capability —
on every request, with 21 tools defined and with none, and no documented variable stops
it. Qwen3.8's template raises `System message must be at the beginning`, llama.cpp
returns that as a 500, and the client retries ten times and dies.
`config/templates/qwen3.8-system-anywhere.jinja` differs from the shipped template in one
line: a non-leading system message renders as its own ChatML block.

**A declared window catches an overflow between turns, not the preamble a session starts
with.** `CLAUDE_CODE_MAX_CONTEXT_TOKENS` makes Claude Code count the conversation through
`count_tokens` and refuse before sending. A first request larger than the window is sent
anyway, and comes back as the server's 400.

**An error whose wording is not Anthropic's costs two documented recoveries.** Claude Code
retries and disables the capability after a mid-conversation-system rejection, and
compacts after a too-long rejection — both by matching the upstream's error text. A Jinja
exception and llama.cpp's `exceed_context_size_error` match neither, so both recoveries
are unavailable here and the corresponding limits have to be declared up front instead.

**No prompt and no host leaves the machine.** Measured with a CONNECT proxy that records
hosts and tunnels TLS untouched: a session doing a real task contacts nothing, and
completes unchanged with every remote host refused. With
`CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` unset the same task makes 9 connections to
`api.anthropic.com` and reaches no other host. A session with no credential refuses to
start, which is a credential problem rather than a reachability one. The claim is scoped
to token authentication: no claude.ai login is stored on this machine, so the OAuth
refresh path is untested.

**The Anthropic→OpenAI conversion changes no result.** The four tool-call fixtures down
`/v1/messages` at three passes are identical to the native path cell for cell — 12/12
valid, 11/12 passing, the same task failing at the same rate. `cmd/eval -api messages`
sends the other dialect to the same grader and refuses sampling flags the Messages body
cannot carry.

**Cursor's built-in assistant was rejected on architecture.** It does not call the configured
base URL from this machine: it routes model requests through Cursor's backend and rejects
plain HTTP. A local model therefore needs a public HTTPS tunnel, and the path becomes
Cursor → its backend → tunnel → here. Code leaves the machine even though inference does
not. The extension hosting Claude Code has none of that, because the agent runs locally.

Client configuration lives in [`harness/claude-code/`](../harness/claude-code/) with the
other harnesses, not here.

## Sessions hand off instead of compacting

A session in this checkout writes `HANDOFF.md`, untracked working state inside one task box.
Three Claude Code hooks carry it: `SessionStart` prints it into the next session's context,
`PreCompact` refuses every compaction, and `SessionEnd` extracts one from the transcript when
the session wrote none. `localcode` is what runs the chain of sessions those hooks carry a
handoff between. Nothing here reads a feature doc's `## Tasks` boxes: judging what a session
delivered is `kit`'s side of the boundary. The wiring is in
[`harness/claude-code/`](../harness/claude-code/README.md) with the rest of the client
configuration.

**A refused compaction does not end the session.** The turn completes and `PreCompact` fires
again on the next one, once per turn. Not only over a threshold: under 0023's gate it fired
before every turn of a session whose context ran 4,325 to 6,183 of an 11,264 window, so a
count of refusals measures turns rather than pressure, and the twenty refusals 0016 opened
with are twenty turns. `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE` was tried against it and changed
nothing, so nothing here sets it. What ends a session is
`CLAUDE_CODE_MAX_CONTEXT_TOKENS` refusing a send it cannot fit, so the refusal buys the
generation a summary would have cost and nothing else — bounding a session is the driver's
job. Measured at 2.1.233, on both triggers.

**`SessionStart` sends `source`, not `session_start_reason`.** All three events fire in a
print session, which is the form the driver runs.

**The pressure this relieves may be the incumbent's own.** Pi ingests a fifth as much per
task, so a harness that spends less of the window may need less of the mechanism. Nothing here
separates the two.

**Whether any of this is worth its cost is unmeasured.** The before/after is one box driven
with the mechanism and without it, and
[0011](features/0011-split-planning-and-grinding-across-frontier-and-local-models.md) is
where boxes are driven locally, so it is where those runs happen. Until then this is
apparatus, not a result.

## One package knows which agent a chain runs in

`harness.Agent` is what the driver needs of a harness: the window it must be told it has,
what one chain writes before its first session, the command for one session, where the
agent files what that session cost, where it keeps its own state so the sandbox can let it,
and how its event stream reads. `cmd/localcode` names no agent, and a test in
`internal/harness` holds that: it parses every file of `cmd/localcode`, `internal/chain`
and `internal/handoff` with the comments dropped and fails on a harness's name, its API,
its event shapes or its files. Comments are exempt on purpose — that is where a decision
records which agent it was taken against.

Which agent ran a session is in that session's `session.json`, so a chain read back long
afterwards is read by the right adapter. A chain recorded before that field existed reads
as the default, which is what it will have been.

What a session cost is read by the adapter that ran it: `Recorder` answers where the
session filed its record, the peak and turns in it, and every call it made. So `localcode
account` reports a Pi chain in the columns it reports an incumbent one in — measured over
two Pi sessions: 2,764 tokens of preamble, 3,119 ingested, 15,853 reused, 585 generated,
107.3 s of 109 s inside a call, decode 7.62 tok/s.

## Pi drives a chain for half the ingest, and the incumbent is displaced at this job

Four chains, two an arm, one instruction each: build a project from scratch out of a
directory holding two months of a private work project's export data, plan its own features,
and work them until none are left. `-harness` is the only thing that differed between the
arms of a pair; both ran the served config's defaults, one at a time against a cold server,
each alone under its own parent directory. Counts only, as
[0032](features/0032-let-a-session-s-call-budget-follow-the-window-it-was-given.md) records
them; the rows are in
[`2026-08-26-m2max-32gb-0040-harness-pairs.jsonl`](data/2026-08-26-m2max-32gb-0040-harness-pairs.jsonl).

| per session, median | Claude Code | Pi |
|---|---|---|
| preamble | 5,464 | **2,925** |
| tokens per tool call | 774 | **597** |
| overshoot past the ceiling | 2,478 | **1,130** |
| tool calls | 34 | 34 |
| generated | 9,283 | 12,820 |

**Pi buys more work with the same window.** It pays 54% of the incumbent's preamble and 77%
of its tokens per tool call, so the same ceiling holds more of the session's actual work —
and it overshoots that ceiling by half as much, which is the reserve
[0037](features/0037-size-the-ceiling-from-what-sessions-do-not-from-what-they-might.md)
sized fitting it better than it fits the incumbent. Pi's preamble is also the steadier
number: 2,131 to 3,188 across seven sessions against 956 to 7,187 across ten.

**One pair finished, and Pi finished it in half the sessions** — 3 sessions and 1h32m
against 6 and 2h49m, both ending on their own work rather than a bound, both answering all
four clauses of the instruction. In the second pair both arms stopped on the 1h session
timeout in session 4 with the same third feature outstanding, so sessions-to-finish rests on
one pair and is a direction rather than a ratio.

**What landed is comparable, and thin in the same place.** Every arm's pipeline runs when
rebuilt from its own tracked source in a clean directory, and every page it emits is
self-contained. The incumbent's finished dashboard is the richer artifact — six charts
against two — and Pi's second chain is the only one that built an executable quality gate
with an exit code. **No arm in any chain wrote a test.**

**This does not overturn [0010](#nothing-displaces-claude-code-and-the-two-axes-disagree),
it splits it.** That sweep scored one turn against a patch fixture and put Claude Code ahead
14/15 to 12/15. A chain is the other job, and at it Pi is cheaper per session on every
column a task mix cannot move. So: **drive a chain with `-harness pi`; reach for the
incumbent for one-shot patch work.** The flag's default is still `claude-code` — flipping
it is a backlog line, not something these four chains alone settle.

**Two caveats travel with this.** Wall clock is not the column: Pi's second chain spent
1,779 s outside what the recorder counts as a model call against the incumbent's 89 s, and
whether that is generation the Pi reader fails to attribute or genuine idle is unsettled.
And the arms only measure the harness where nothing else is in reach — an earlier Pi chain
that could see a sibling checkout solving the same task spent all three of its sessions
reading it, which is a fact about the fixture and not about Pi.

## A session is budgeted rather than left to fill up

0016 made a full context survivable; this keeps a session from reaching one. Every session
`localcode` starts carries a budget derived from the window the harness was declared, and
two hooks enforce it — `localcode hook gate` on `PreToolUse`, `localcode hook stop` on
`Stop`. Neither asks the model for anything, because instruction was measured not to work:
a session told in prose to spend three commands reached compaction anyway, and one warned at
45% of its window acknowledged the warning and carried on.

**Pi is held to that budget by the same binary.** A `tool_call` handler in
[`harness/pi/localcode-gate.js`](../harness/pi/localcode-gate.js) fills the gate's payload
from Pi's vocabulary and runs `localcode hook gate`, so the two harnesses cannot hold a
session to two budgets. The context comes in on the payload as `peak_tokens` rather than
off a transcript: Pi reports its own, and that reading sums the same four usage fields a
Claude Code transcript is read for.

**That session cannot compact.** `session_before_compact` cancels every trigger — manual,
threshold and overflow — and
[`harness/pi/settings.json.reference`](../harness/pi/settings.json.reference), seeded into
the `PI_CODING_AGENT_DIR` a run is given, serves a compaction reserve of 0 so the threshold
sits at the whole served context rather than 16,384 tokens below it. Measured at an
8,192-token window with the recent-token floor at 100: the default reserve compacted a
two-prompt session twice, the served reserve compacted it none, and the cancel alone
compacted it none.

**And it cannot end having handed nothing on.** `localcode hook stop` answers an
`agent_end` handler, and its refusal is delivered as a follow-up message rather than an
exit code, because a message queued there is what Pi continues on. The grace is the Stop
hook's two refusals: measured, a session given `write` answered the first refusal with a
handoff, and one given only `read` was refused twice and then allowed to end.

**The ceiling is derived from what lands after it.** The gate decides on the context as the
transcript last recorded it, and two things arrive after that reading: the results of the
calls it is permitting, and what the turns that follow them generate. So the ceiling is the
window less a quarter for results and less 6,144 tokens for the turns, and that is also the
default: as high as the arithmetic allows and no higher. The window it is a fraction of is
the served context less what a reply may generate, which is the whole of what the server
will take. `-ceiling` only lowers it, and lowering it buys no safety the reserve does not
already buy while costing a handoff — measured at half the window, a session could not both
read a file and edit it, which `Edit` requires of it, so the chain wrote handoffs and never
changed a line. A window with no room left for the preamble is refused rather than clamped:
a session started in one spends a cold ingest to say `Prompt is too long`.

**The two terms are bounded differently, and only one of them by the gate.** A quarter of
the window is exactly the four maximum-sized results a turn is permitted, so that term is a
bound the gate itself creates. Nothing bounds what the turns generate except the reply the
harness would allow, and the reserve held twice that — 8,192 tokens — for turns measured at
2,774 in the worst session of eight chains. The turn term is therefore the measurement below
and not the bound: twice the worst recorded, which leaves the whole reserve at 2.1x the
largest growth ever seen past that reading, at both windows this project serves. It buys
2,048 tokens of ceiling and costs the smallest contexts: a served context under about 24,576
now leaves too little to read a file, change it and see what that did, and is refused rather
than run.

**What lands after that reading, measured across every chain here.** Eight chains and 69
scored sessions — five against the generated fixture, three against a private work
repository — and 54 of them ended on the ceiling rather than on a call budget or on their
own work. The overshoot past the ceiling runs 342 to 5,486 tokens with a median of 1,956,
and the session that came closest to the wall left 12,946 tokens of window unspent. Read
from the transcripts of the 51 whose state directories survive, what arrives after the last
model call at or under the ceiling is **at most four turns**: they generate 1,450 tokens at
the median and 2,774 at the worst, and grow the context by 2,396 and 7,881. So the two
reserve terms are not equally wrong. The results reach 72% of the four-maximum-sized-results
bound the gate itself creates, which no run has spent; the turns reach a third of twice the
output reservation held for them, and nothing in the distribution comes near it. The rows
are in
[data/2026-08-25-m2max-32gb-0037-overshoot.jsonl](data/2026-08-25-m2max-32gb-0037-overshoot.jsonl).

**The harness's own context check is off for the sessions `localcode` starts.** It is a
second answer to a question this repo has already answered: it exists to stop a session
sending a prompt the server will refuse, which is what the gate does — from a transcript
reading, against a ceiling, with a handoff on the other side of the denial. Left on it holds
back about a quarter of whatever it is told, by an undocumented fraction, so a window derived
from it is a window nobody can account for. With
`CLAUDE_CODE_DISABLE_UNKNOWN_MODEL_WINDOW_ENFORCEMENT=1` the window is what the server will
take — measured, a 44,509-token prompt is sent against a 49,152 context where the check
refused anything past 34,258. `harness/claude-code/claude-code.env` still leaves it on,
because a session somebody runs by hand has no gate.

**What replaces it is llama-server's own 400, and the driver records it.** `request (49509
tokens) exceeds the available context size (49152 tokens)` — both numbers, so a session that
overran says by how much. The harness surfaces the body verbatim and does not recover, so
the supervisor reads it out of the event stream and writes it to the session's row as
`context_overrun`. One appearing at all means the budget was wrong: the ceiling and the
reserve exist so that no prompt this driver sends can reach it.

**The harness sends at most three-quarters of the context it was declared, less its
reservation — with its own check left on.** Bisected at four declarations against one 49,152 server, by padding a prompt
to an exact served-token count and reading whether it was sent: at 49,152 declared with
4,096 reserved the largest prompt that went through counted 34,008 tokens and the smallest
refused 34,258 — 0.755 of the 45,056 the arithmetic here calls the window, and 0.692 of the
context. The other three walls sit at 0.768, 0.794 and 0.757 of that same quantity, so
**three-quarters is under every one of them** while `declared − reservation` is above all
four. The refusal is the harness's own — `Prompt is too long`, exit 1, and nothing reaches
the server — which is what makes the measurement cheap in one direction and a cold ingest in
the other. The rows are in
[data/2026-08-25-m2max-32gb-0037-prompt-wall.jsonl](data/2026-08-25-m2max-32gb-0037-prompt-wall.jsonl),
and [`scripts/promptwall.sh`](../scripts/promptwall.sh) re-runs it.

**With the check on, the output reservation is subtracted twice, and it is the smaller
mistake.** The
largest prompt the harness will send at a declared 49,152 is 34,008 tokens; with the whole
4,096 reservation on top that is 38,104 of the 49,152 served, so declaring the served
context in full cannot overrun it — the same prompt is refused at a declared 48,128 and sent
at 48,640, which is the wall read the other way round. What the declaration was hiding is
larger: at the shipped declaration of 45,056 the window was taken as 40,960 while the
harness would not send past about 31,200, so every session was budgeted against a wall
10,000 tokens further out than the one it would actually hit.

**A turn is bounded as well as a session, at four calls.** One transcript reading otherwise
decides a whole turn's calls, because the harness issues them together and nothing changes
while they run.

**Measured end to end.** Eight independent Go bugs, one per file, at a 16,384 wall — a
12,288 prompt budget and a 7,168 ceiling. Three sessions, 1,115 s, 20,565 tokens ingested,
all eight tests passing, and the first two both denied at their ceiling mid-work so no
single session could have done it. Peaks of 10,090, 8,819 and 5,834 against a 12,288
budget: the overshoot past the ceiling is about 2,900 tokens, which is what the
quarter-window reserve is for. Each handoff carried results — which files were fixed, which
test still failed, what `go test` said — and the rows are in
[data/2026-08-22-m2max-32gb-0023-chain.jsonl](data/2026-08-22-m2max-32gb-0023-chain.jsonl).

**A handoff cannot carry the right to edit.** `Edit` fails on a file the session has not
`Read`, so every session in a chain pays the read for every file it changes, however well
the handoff describes it. A read and an edit cost about 350 tokens a file at a 12,288 wall,
which is what sets how many files a session can get through before its ceiling. The
appended system prompt says so, because a session that surveys before it acts spends its
whole ceiling on reads it cannot follow up — measured, three times over.

**A tool call costs about 680 tokens, and a generated fixture says a third of that.** Over
the 35 scored sessions of the longest chain run here — one instruction against real source,
at a 22,528 ceiling and at 10,240 — the context a session reached above its preamble divided
by the calls it spent has a median of 671 and a call-weighted aggregate of 683, spread from
335 to 1,474 with quartiles at 579 and 980. The six fixture sessions of 0023 and 0025
aggregate to 298, so **a number taken from generated bugs in one-function files is 2.3x too
cheap** for the work a chain is actually pointed at. The cost also moves with the ceiling —
859 a call at 10,240 against 597 at 22,528, by session median — because a session's first
calls carry a fixed orientation the later ones amortise. The rows are in
[data/2026-08-24-m2max-32gb-0032-call-cost.jsonl](data/2026-08-24-m2max-32gb-0032-call-cost.jsonl).

**That is what makes one flat call budget wrong at both ends.** The room above the preamble
buys about 8 calls at a 10,240 ceiling and about 26 at 22,528, against the 30 both were
given: at the smaller ceiling no session of the seventeen reached the budget and their
median was 9 calls, while at the larger eleven of eighteen ended on it.

**So the budget follows the ceiling, at the cheapest call rather than the typical one.** It
is the room above the preamble divided by 350 tokens — 66 calls at the shipped context, 30 at
32,768 — and 350 is the low end of the measured range because this bound is the backstop for
when the transcript cannot be read: sized on a typical call it would pre-empt the ceiling for
every session whose calls come cheaper than typical, which is half of them. A ceiling with
room for fewer than one turn's four calls is refused rather than served, on the same argument
the preamble floor already makes. `-calls` overrides the derived number and is taken as
given, below the floor included: a measurement the tool rounds up to what it thinks
reasonable is not one.

**Both unbounded tools are capped, each where it can be.** `BASH_MAX_OUTPUT_LENGTH` is set
to a sixteenth of the window, so one unbounded command cannot spend a session inside a
single permitted call. `Read` has no such setting — its own bound is two thousand lines,
which bounds lines rather than the window — so the gate narrows the call instead: a
`PreToolUse` hook may rewrite a tool's input, and `Read` takes a structured `limit`. How
many of a file's lines fit in the reserve is computed from the file, not from a
tokens-per-line guess. Measured against a 108,893-byte file in a 12,288-token window: the
model asked for `limit: 1200` on every call — 65 KB apiece — and each came back at about
3,230 bytes against a 3,072-byte reserve, the excess being the line numbers the harness
adds. Unclamped, the first call would have ended the session.

**A session shows its work.** `claude -p` prints its result and nothing before it, so a
chain was minutes of silence between summaries — one measured session spent 591 s before it
said a word, which is indistinguishable from a wedged run. A one-shot session is asked for
`--output-format stream-json --include-partial-messages` instead and the supervisor renders
it: a line per tool call as it happens, the model's prose a token at a time as it is
generated, and a line for every call the gate refuses. Token by token rather than message by
message, because at 5-10 tok/s a paragraph is a minute and a minute of nothing is
indistinguishable from a wedged run. An interactive session is untouched, because the
harness draws its own screen there.

**Claude Code's prompt budget is the declared window minus `max(MAX_OUTPUT, 4096)`.** It
keeps 4,096 for a reply whatever it is told to keep, so `CLAUDE_CODE_MAX_OUTPUT_TOKENS`
below that buys nothing back. Bisected by padding a prompt to an exact token count — the
server's own `/tokenize`, not a character estimate — and reading whether it was refused
before it was sent, which costs no model time at all. Against a declared 12,288: with 1,024
reserved the boundary falls between 3,700 and 3,900 tokens of padding on top of a
~4,390-token preamble, putting it at 8,192; with 6,000 reserved it falls between 1,500 and
2,500, which is 1,800 lower against the 1,904 the rule predicts.

**Taking the declaration at face value is what killed the sessions this was found on.** A
budget 3,072 tokens too generous let sessions edit four files each and then die on `Prompt
is too long` with no handoff written. The declared window has to clear the preamble plus
that reservation plus the reserve, or `localcode` refuses to start — which rules out the
12,288 wall the enforcement was first measured at.

**And the declaration is read off the server, not off the file.** `harness/claude-code/`
carries one `CLAUDE_CODE_MAX_CONTEXT_TOKENS` and `-config` chooses which context is served,
so a number written for one server is a claim about the other. `localcode` asks `/props`
what is served, declares that less the output reservation, and overrides the variable for
the session it starts — saying so when the two disagree. A server that will not report a
context is refused rather than guessed at. The file's own value stands for the flow it is
sourced into by hand.

**An interrupt reaches the session, because the supervisor owns the group it is in.** A
background session runs in a process group of its own and the supervisor forwards the
signal to it — forwarded and not killed, so the harness's own shutdown runs and the handoff
still lands. Relying on a shared group holds only at a terminal, where the signal goes to
the foreground group: a chain started in the background and signalled by pid took the
supervisor alone, and the session it was waiting on ran on against the endpoint until it
was matched by its `--add-dir` argument and killed by hand. A session at a keyboard keeps
the terminal's own group, which is what delivers its interrupt and what it must stay in to
read at all. **A second interrupt kills that group**, because the reason to send one twice
is that the first was ignored, and a supervisor that cannot be stopped is worse than a
session that dies mid-edit. The session's clock kills the same group, and for a
reason the group made visible: `sandbox-exec` execs the harness, so a kill by pid reaches
the harness and not what it started, and a surviving `Bash` call or hook holds the pipe the
supervisor is reading — measured, a one-second timeout returned after ninety-five.

**A resume on a different budget says so before it runs.** What a session is budgeted
against is whatever the server serves at the moment it starts, and a chain resumed on the
default config took a 10,240 ceiling where its sessions before had 22,528 — two starved
sessions ran before anybody read `session.json` by hand. The supervisor compares what it is
about to use against what the chain's last session actually ran under, and names every bound
that moved along with the endpoint now serving it. Narrated rather than refused: a smaller
window is a legitimate choice and sometimes the only one the machine has, and what is wrong
is making it in silence. The session bound is not compared, because a resume is what raises
it and a warning that fires every time is one nobody reads. The difference goes into `chain.json`
beside the ending as `budget_changed`, because the terminal a background chain was resumed
from is the one place the warning cannot be read back from.

**A session is told when the plan it inherited has not been landing.** It sees that
handoff and nothing else, so a plan that has already failed twice reads exactly like one
that has not — measured, a session concluded the sandbox was corrupting its regexes and the
four after it inherited that as a premise and spent 2h44m without a commit. After two
sessions that committed nothing the next one is told the count, in the appended system
prompt beside the handoff briefing: the same words through a tool result were refused as
injection, correctly. A fact rather than a request, which is what separates it from the
prose measured not to work here — that asked a session to want less than it wanted, and
this reports something the session has no other way to know. The count is read from the
rows, so a resumed chain carries what the old plan cost, and the sessions told are recorded
in `chain.json` as `nudged_sessions`. **Whether it works is unmeasured**: the generated
fixture cannot reproduce a chain talking itself into a false premise, so the record is what
a later reading rests on.

**A chain stops when the repository stops moving, not when its prose repeats.** Three
sessions in a row that leave the repository as they found it end it — three and not one,
because a session that reads before it edits is normal, and not two because two was matched
to the `Next` comparison's patience before any chain had been watched: on one of ten, four
sessions were still and no pair of them was a stall. The `Next` comparison stays as the
first test and catches an identical pair on sight; what it cannot see is the failure that
motivated this, where eight consecutive sessions of a 25-session chain committed nothing
while each reworded the same plan, costing about 49 minutes. Movement only judges a chain
once that chain has moved a repository at all, so work that leaves no trace — a
measurement, an investigation — is judged on its handoffs as before.

**A session reports whether the repository moved while it ran.** Its row carries
`repo_moved`, which is the object database's size and **every worktree's** state read
either side of the session: a commit made in any linked worktree lands in the objects they
share, so a chain working in a claimed one is measured even though the checkout's `HEAD`
never moves, and a session that only read has moved nothing. The working-tree half reads
them all because a project worked through `kit` edits in a linked worktree — reading the
checkout's alone, a session that wrote 140 lines across two files and ended before
committing recorded `repo_moved: false`, and two of those stopped a ten-session chain that
was converging. The field is absent rather than
`false` outside version control, because a chain with no repository to read has not been
measured.

**A commit is recorded apart from movement, because they answer different questions.** The
row also carries `repo_committed`, which is what `git rev-list --all --count` says either
side of the session: a commit made on any branch counts, and a branch made at one that was
already there does not. A session writing throwaway probes moves a worktree without leaving
anything durable, so movement is the right test for whether a session did anything and the
wrong one for whether a chain is landing work — measured, commits stopped five sessions and
2h44m before movement did.

**A chain is one invocation, and a repository holds several.** `localcode` given an
instruction runs sessions until a handoff says `Next: none`, until two in a row plan the
same step, or until `-sessions` runs out, and each of the three says which happened. It
writes which it was to `chain.json` beside its rows — one of `finished`, `stalled`,
`bound`, `timeout` or `interrupted`, the session it happened at, and the handoff to open —
so a run whose narration went to a stream nobody kept is still readable afterwards. The
step is read from the `**Next:**` line or from the lines under it, and a handoff carrying
none is neither an ending a session may take nor a step a later one can repeat. The words
that end a chain are `none`, `nothing` and `done`; `no` ends one only with a qualifier
after it, because the step is read up to its first punctuation and `No, the tests still
fail` reduces to the same word as `none`.
Starting clean is the default, `-continue` takes the newest chain, `-resume` takes one by
id, `-fork` starts a new one from what another knew, and `localcode sessions` lists them
with how each one ended.
One handoff per repository was wrong: a second instruction in the same checkout would have
resumed the first and then overwritten what it knew.

## A chain's clock is its model calls, and half its ingest is preamble

`localcode account [id]` reports what one chain cost, off files the run already left:
`sessions.jsonl` for the wall clock, each session's `session.json` for the budget it ran
under, and its transcript for the tokens. `-jsonl` appends the same as rows. It needs no
server and waits for nothing, so a chain still working is read while it works — its running
session has no row in `sessions.jsonl` yet, and the numbered directories are what say a
session exists.

**A call is one response, not one transcript row.** Claude Code files an assistant row per
content block and every one of them carries that call's usage, so a turn answering with
text and two tool calls is charged three times unless the rows are folded on their message
id. Folded, `prompt_tokens_ingested` and `tokens_generated` reproduce the server's own
counters to the token on both chains of
[0025](data/2026-08-23-m2max-32gb-0025-chain.jsonl).

**Wall clock outside a model call is noise on a driver chain**: 5.2 s of 661 at 32,768 and
2.9 s of 825 at 49,152. Tool execution and the harness's own work are under 1%, so what a
chain costs is what its calls cost, and nothing is hiding between them.

**Half of what a chain ingests is preamble.** The driver chain paid 4,165 then 4,430 tokens
of the 17,119 it ingested — 50%, because a preamble is paid once per session and a session
is what a handoff creates. That is the term a shorter context multiplies, and it is why
adding a session is not free even where the ingest per turn is small.

**The two rates are fitted, and they are the only derived numbers here.** A transcript
records no first-token time, so the split of a call's clock between its prompt and its
reply is a least-squares fit over the chain's own calls rather than the client-side
measurement [the scorer takes](#speculative-decoding-adoptable-at-the-top-of-the-context-not-the-bottom).
`localcode account` refuses to print a pair the calls cannot separate. Read as a ratio it
agrees with what was measured another way — 8.41 tok/s against 6.44, which is the 1.31x the
MTP head was adopted for; read as an absolute rate each figure carries the harness's own
time around the call and is the slower for it.

## The two-tier split, measured

A local builder took a real feature from a frontier-drafted doc and shipped a reviewable
branch using `kit` unchanged. **Nothing in the protocol needed changing for a 27B model to
follow it** — the branch and the doc's boxes were already the whole interface.

**Two things were needed and neither was sufficient**: 0016's hooks, which remove the
compaction tax, and a restricted tool set, which returns **18,045 tokens — 40% of the
window** — because Claude Code defines 21 tools unless told otherwise. With the mechanism and
no room the session produced nothing; with room and no mechanism it finished only after
compacting. The instruction that worked was the one the `SessionStart` hook already prints:
the job and the size of the window, not how to work.

What each tier costs, from the harnesses' own transcripts. The windows are stated because
they are the weak part of the measurement — a window holds everything done in it, not only
the feature.

| | turns | tokens in | tokens generated |
|---|---|---|---|
| **0010, frontier throughout** — 12 boxes, 5 h | 468 | 138,772,396 | 539,327 |
| **0015 box 1, frontier drafting only** — 15 min | 50 | 22,902,916 | 66,152 |
| 0015 box 1, local builder finishing it | 68 | 1,532,612 | 12,147 |

**Per box the frontier tier spent about 45,000 generated tokens on 0010 and about 16,500
drafting all four of 0015's**, and the box was then built for 12,147 that cost nothing. The
doc is the frontier tier's whole contribution and is written once for every box in it.

**Three things stop that being a clean multiple.** The features are not the same size — 0010
carried a 60-run sweep and 0015's box is one function. The frontier figure excludes review,
which is real and recurring. And input tokens are not the cost they look: the frontier column
is dominated by cache reads, and the local column by a conversation resent whole every turn,
which at 49,152 is time rather than money.

**Where the split inverts.** The first local build produced a check that could never pass — a
floor on free memory, which reads 0.16–0.65 GB here whether a run swaps or not. The local tier
built exactly what the doc specified, and the doc was wrong. Finding that and correcting the
premise cost a frontier session — **more than drafting the doc had.** So the split pays where
the specification is sound and the work is mechanical, and inverts where the specification is
the hard part. That is the boundary the per-task record draws, from the other side.

**What the frontier tier is shown: the whole repository — docs, code, git history.** Decided
by the human this project is for. It costs little to permit because secrets never enter a doc
and the measurement data is readings from one laptop. The boundary is the repository and not a
default: nothing outside the checkout is planning context, and a drafter that needs something
from outside asks for it to be committed, which leaves a record of what was shown. That is the
vision's named exception used in full — planning leaves the machine, grinding does not.

## What the local tier finishes unattended

Drawn from the per-task record rather than from judgement: every tier-1 fixture at the
settled config — thinking off, model-card sampling, llama.cpp — and 0010's 60 agent-loop
runs. The two tiers agree on where the line falls.

**Twelve of the fourteen tier-1 fixtures score 100%, and three of the five agent-loop
fixtures are 12/12.** Mechanical single-file work is finished without supervision, in one
request or in three turns: a nil guard, a boundary condition, a nil-map init, reading or
editing a named file, recalling a key at depth.

Two classes are not, and neither is mechanical:

| class | tier 1, settled | agent loop | what it actually asks for |
|---|---|---|---|
| the spec contradicts itself | 7/9 | 9/12 | deciding which authority wins |
| the right action is to refuse | 8/12 | not covered | declining instead of acting |

`patch-contradiction-rounding` sets a doc comment against a test that must keep passing:
the model satisfies one and does not reliably notice the other. `toolcall-constraint-readonly`
fails as `fail_wrong_tool` — told it may not write, it writes anyway, four times in twelve.
**Both are judgement about which instruction governs, which is the frontier tier's half of
the split.**

A third class belongs to the harness rather than the model: **an in-place edit that must
preserve what it did not write.** `patch-sibling-splitpath` is 9/9 as a whole-file rewrite in
tier 1 and 9/12 through the agent loop. Every one of those three failures is an edit-based
harness dropping the `strings` import. Claude Code and Hermes, which rewrite the
file, took it 3/3. 0010 settled on Claude Code, so this class sits inside the boundary as
configured — and outside it for anyone who switches to an edit-based harness.

**The evidence stops at single-file changes.** Every fixture in the suite is one file and
one function, and the instruction is one sentence. Nothing here says what the local tier
does with a change spanning modules, or with a specification long enough to hold its own
contradictions. That is what a feature tests, and it is measured rather than extrapolated.

## The local model runs in any repository, sandboxed

`localcode` is this configuration installed: `make install` builds it with the checkout's
path compiled in, and it drives the agent from whatever repository the developer is
standing in. Nothing searches for the checkout, because a launcher that guesses picks the
wrong one as soon as there are two — which is the normal state here.

**An unrestricted `Bash` tool is defensible only because the boundary is the kernel's.** A
command allowlist cannot be generic across ecosystems and a denylist of dangerous strings
is defeated by `sh -c`, so neither is a boundary. `sandbox-exec` needs to know nothing about
the language in the repository. Writes reach the working directory, temp, the two cache
roots and the agent's own state; reads are unrestricted, because an agent that cannot read
a toolchain cannot use one. Go, Python, Node, `make` and `git` all complete under it.

**The network is loopback-only**, so a repository's source cannot leave the machine and
`curl | sh` fetches nothing — VISION's offline property enforced rather than configured.
`-net` lifts it for one session and widens reachability, never the filesystem.

**A denied write is explained by the session, not by the launcher.** claude gives a tool's
stderr to the model rather than passing it through, so the process that could print a hint
is the one that never learns the write was refused. The sandbox is described in the
appended system prompt instead, and `~/.config/localcode/writable` is where a developer
names the paths their ecosystems need — the tool learns no language.

**Session state stays out of the repository being visited**, under
`~/.local/state/localcode/repos/<slug>/`, keyed by the repository's path. 0016's hooks take
`LOCALCODE_HANDOFF_DIR` and still default to the checkout, so working on localcode itself is
unchanged.

**A worktree goes in `.worktrees/` or beside the repository, and the session is told
which.** `.worktrees/` inside the checkout was always writable — it is under the working
directory — but nothing said so, and a session that met `Operation not permitted` on
`../<repo>-<name>` put the worktree where nobody looks for it instead. The appended
briefing now names both places, `.worktrees/` first where the repository already keeps one,
because a repository with that directory has decided where they go.

**A worktree beside the repository is writable; the parent is not.** `kit claim` prints
`git worktree add ../<repo>-<id>`, which the working-directory-only policy refused with
`Operation not permitted` on a real repository — the session recovered by putting the
worktree inside the repository, which works and is where nobody looks for it. The profile
now carries a regex for siblings named after the repository, escaped, and resolved like
every other path. The parent stays closed, because it is where every other project lives.

## Gotchas

Each of these has already caused a wrong number in this repo.

- **Free memory is not a pressure signal on macOS.** It sits at 0.04–0.08 GB whether the
  machine is idle with zero swap or deep in paging. Use swap used, swap delta and
  compressor size. An entire "the config does not fit" argument was built on free memory
  and was wrong.
- **`vm_stat` counts pages, and this machine's page is 16 KB.** `memprobe.sh` divided by a
  hardcoded 4096, so every free and compressor figure recorded before 2026-08-18 is
  understated fourfold — see the erratum in [data/README.md](data/README.md). It reversed
  no conclusion, because free memory is pinned near zero at any scale factor, but it would
  have corrupted wired memory the moment 0014 read it off the same counter to decide which
  configs are admissible. Read the size from `pagesize`, never assume it.
- **`ps rss` is clamped by what physically fits, not by what a process wants.** Under
  pressure it stops distinguishing configs exactly where the answer matters — peak RSS
  moved 0.82 GB across a fourfold context range while saturated, and more once pressure
  was gone. Time-to-ingest discriminates where RSS does not.
- **seatbelt matches resolved paths, and `/var`, `/tmp` and `/etc` are symlinks into
  `/private`.** A profile naming an unresolved `TMPDIR` denies every compiler that uses one
  while appearing to allow it, and the failure reads as a broken toolchain rather than as a
  policy. Resolve every path before it reaches the profile.
- **A test that names one platform's symlink is testing the platform.** The sandbox profile
  must carry resolved paths, because seatbelt matches the resolved path and `/tmp` is a
  symlink into `/private` on macOS. Asserting that the literal string `/tmp` is absent says
  "resolved" only where `/tmp` resolves to something else: on Linux it resolves to itself,
  so the assertion failed CI over a profile that was correct. State the property — the path
  in the profile is its own resolution — and it holds on both.
- **A completion signal the model must phrase exactly is one it will phrase otherwise.**
  A chain ends when a handoff's `**Next:**` says `none`. Measured on real work, a finished
  session wrote `none — 0003 is done. Remaining: human merges…`, which an exact match read
  as unfinished: the chain spent another whole session and then reported its `-sessions`
  bound rather than its success. The test is now the first clause of the line, with a short
  allowlist of qualifiers, because "none of the tests pass" is the opposite of done and
  begins the same way.
- **A sandbox test written inside the temp directory proves nothing.** `os.TempDir()` is
  writable by the profile, so a refusal asserted against a fixture under `t.TempDir()`
  passes whatever the policy says. Put the fixture under the home directory, where the
  denial is real. The test that found this was asserting the parent of a repository stays
  closed; it was writable for a reason that had nothing to do with the rule under test.
- **Characters over four is not a token count.** A budget probe that padded prompts by
  `chars/4` bracketed Claude Code's limit at 9,400–10,200 tokens; the same probe padded
  through the server's `/tokenize` put it at 8,192. The first number was wrong by a fifth
  and it was believed for an afternoon, because it agreed with an arithmetic that was also
  wrong. The server has a tokeniser and it is one HTTP call away.
- **A hook fires once per tool call, and one turn's calls run at once.** Two bugs came out
  of that in one afternoon. A counter kept by read-modify-write loses calls — eleven
  permitted left one reading eight, so a budget silently allowed half again as much as it
  said. And a hook that reads the transcript reads the same numbers for every call in a
  turn, so one decision admits a whole batch: a five-call turn carried the context 1,960
  tokens past a ceiling it had been under when the gate looked. Append a byte and decide on
  the offset the write returned, and bound the turn as well as the session.
- **A hook's prose is part of the mechanism.** Relocating the handoff's state was not
  enough: the SessionStart text still told the model to create it "at the root of the
  checkout", so the model did, in the repository being visited. Moving where a file is
  written means changing what the session is told about it.
- **A health check must not conclude "dead" before the process exists.** `serve.sh`
  validates its config and only then `exec`s llama-server, so for the first instants after
  launch there is nothing for `pgrep` to find. A poll that bails the moment the process is
  absent — curl refused in a millisecond, pgrep finding nothing — records `load_failed` for
  a server that goes on to load in 15 s, and leaves it running to collide with the next
  cell. It fired only once the machine carried 20 GB of other processes, where the child is
  slow to be scheduled: a race that hid through every earlier run appears exactly when the
  measurement gets interesting.
- **`cmd && run || echo "skipped"` reports a failure as a skip.** The lint target used that
  shape to tolerate a missing binary. With golangci-lint present *and finding issues*, the
  non-zero exit took the `||` branch: `make check` printed "not installed, SKIPPED" and exited
  0 while CI failed on the same findings. The local gate was green for two pushes that CI
  rejected. A fallback must be reachable only for the condition it describes — write it as an
  `if`, not as the right-hand side of an `||`.
- **Killing an 18 GB server is not instant.** A fixed `sleep` after `pkill` lets the next
  server fail to bind while the health check passes against the *old* one, silently
  measuring the previous config under the next config's name. Poll until the process is
  gone, then verify `/props` reports the context you asked for.
- **A cap that binds has three possible causes, not one.** The fixture underbudgeted, the
  model did not terminate, or the reasoning level was wrong — and a single row cannot tell them
  apart. Raising the cap and watching the reasoning distinguishes the first two: bounded need
  converges, a spiral scales with the budget. Doing that here turned "the fixture is
  underbudgeted" into "the default effort level does not terminate", which was the real answer
  and a different fix entirely.
- **A shared `max_tokens` starves thinking mode.** Reasoning is charged against the same
  budget as the answer, so a cap sized while testing with thinking off produces empty
  answers and looks like a quality failure. Detect `finish_reason == "length"` separately;
  never score a truncated answer as a wrong one.
- **`-hf` downloads more than the weights.** It pulls and loads an 888 MB multimodal
  projector when the repo ships one, costing 1.02 GB resident that text-only coding never
  uses.
- **The HuggingFace cache stores snapshots as symlinks into `blobs/`.** BSD `stat -f%z`
  does not follow them and reports the link's own size, which reads as a 0 GB model and
  silently inflates any headroom estimate built on it. Use `stat -L -f%z`.
- **Resident size and swap cannot see GPU-wired pressure.** A config can leave swap
  untouched, report a comfortable resident size, and still starve the compositor:
  `iogpu.wired_limit_mb` is a separate budget and Metal buffers come out of it. **It
  defaults to two thirds of physical memory here, not the three quarters this repo assumed
  for its first month** — 21,845 MiB of 32 GiB, read from Metal's recommended maximum
  working set by [`scripts/gpulimit.sh`](../scripts/gpulimit.sh). The sysctl answers `0`,
  which means the kernel derived a limit and will not print it, not that there is none. The symptom is a glitching desktop, not a slow model —
  and the model reports success throughout, because it is the process that got the memory.
  **That it binds is measured rather than asserted**: the config that fails the allocator at
  21,845 MiB serves a 44,236-token prompt at 24,576, which
  [`scripts/gpuraise.sh`](../scripts/gpuraise.sh) applies and undoes in one command. A raise is
  a sysctl, so a reboot is the way back out of one.
- **A pass criterion that only asks about the model is blind to the machine.** 0003's
  ladder marked 64k `ok` while that context made the desktop unusable. Any measurement
  meant to protect the machine has to take its verdict from outside the model process.
- **Fixture Go files are `.go.txt`.** Named `.go` they sit inside this module, so
  `go test ./...` compiles the deliberately-broken fixtures and the gate goes red.
- **`cmd.Dir` does not update `PWD`, and Go replaces the environment wholesale when `Env` is
  set.** A tool that resolves its project from `PWD` then works in the launching directory —
  OpenCode reports that as "Unexpected server error". Relative config paths fail the same
  way, resolving against the scratch checkout, so paths are made absolute at validation.
- **zsh does not word-split unquoted variables.** Scripts here run under `bash` via
  shebang; a loop written interactively in zsh can produce config filenames with the value
  glued in, which the ladder will then glob and parse.
