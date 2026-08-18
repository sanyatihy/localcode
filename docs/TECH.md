# TECH — as built

Durable facts about what this repo actually does, moved here as features ship.
Design arguments stay in the feature docs; this is the state of the machine.

## Serving

`scripts/serve.sh <config>` starts `llama-server` from a config file and adds no
flags of its own. `make serve` runs it against `config/baseline.env`;
`make serve CONFIG=config/<variant>.env` runs a variant. Later features add a file
per variant rather than editing the baseline — that is what keeps 0003's ladder and
0004's grid mechanical instead of hand-run.

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
`/v1/messages/count_tokens`, converting to chat-completions internally. Claude Code
can therefore point at this server with `ANTHROPIC_BASE_URL` and no proxy — see
[0008](features/0008-wire-the-winning-config-into-the-coding-agent.md).

## Checks

`make check` is the offline gate and is what CI runs: `gofmt` cleanliness, `go vet`,
and `go test -race`. It needs no server and no model weights, because CI has neither.

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

**The two ranges are different, and both are real.** The model serves 8k–64k. A machine
someone is using is admissible to **56k**; 64k is unattended-only. Every cell above
completed its request, so nothing but the desktop column distinguishes the last row.

The compositor **stalls** rather than saturates — the failing cell's peak sits below every
passing cell's minimum, so the populations do not overlap — and it is flat from the first
sample of the cell, which points at allocation time rather than at ingest.

**The mechanism is not yet established.** Wired peak moves only 0.54 GB across a doubling of
context, and the failing cell had 1.71 GB of headroom against an assumed 24 GB limit where
the passing cell had 1.82 GB. That difference cannot explain a collapse, so either the limit
sits near 22.3 GB rather than 24 — it is `default-assumed`, never read — or something other
than the cap is binding. 0014's raise experiment separates the two.

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
| Thinking mode | 3.06× completion tokens, 1.67× wall, on tier-1 tasks |

### Ladder rungs are derived, not written down

`scripts/rungs.sh` computes which contexts to ladder over from the machine: total memory,
the weights file's actual size, a reserve for the desktop, the measured KB/token, and an
ingest-time budget. It reports **which of memory or time binds**, which is the ladder's
whole question.

On this machine it derives 8k/16k/32k/64k — the same rungs that were first written by
hand — and reports ingest time as the binding constraint. Modelling 128 GB with
`TOTAL_GB=128` gives **the same rungs**, and so does a 70 GB model: memory allows ~262k
tokens in every case while a 20-minute ingest budget allows ~64k.

That is worth stating plainly: **more RAM does not buy more context for this model.** It
buys larger quants and larger models. Context is bounded by ingest time, and ingest time
does not care how much memory is spare.

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
- **A health check must not conclude "dead" before the process exists.** `serve.sh`
  validates its config and only then `exec`s llama-server, so for the first instants after
  launch there is nothing for `pgrep` to find. A poll that bails the moment the process is
  absent — curl refused in a millisecond, pgrep finding nothing — records `load_failed` for
  a server that goes on to load in 15 s, and leaves it running to collide with the next
  cell. It fired only once the machine carried 20 GB of other processes, where the child is
  slow to be scheduled: a race that hid through every earlier run appears exactly when the
  measurement gets interesting.
- **Killing an 18 GB server is not instant.** A fixed `sleep` after `pkill` lets the next
  server fail to bind while the health check passes against the *old* one, silently
  measuring the previous config under the next config's name. Poll until the process is
  gone, then verify `/props` reports the context you asked for.
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
  `iogpu.wired_limit_mb` is a separate budget, defaulting to about 75% of physical memory,
  and Metal buffers come out of it. The symptom is a glitching desktop, not a slow model —
  and the model reports success throughout, because it is the process that got the memory.
- **A pass criterion that only asks about the model is blind to the machine.** 0003's
  ladder marked 64k `ok` while that context made the desktop unusable. Any measurement
  meant to protect the machine has to take its verdict from outside the model process.
- **zsh does not word-split unquoted variables.** Scripts here run under `bash` via
  shebang; a loop written interactively in zsh can produce config filenames with the value
  glued in, which the ladder will then glob and parse.
