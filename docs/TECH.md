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

The desktop verdict is fixed in advance rather than read off each run: `fail_saturated` at
a sustained ≥ 0.90 cores over any 30 s window, `fail_stalled` at ≤ 0.02, `pass` at neither
over a run of at least 30 s, and `not_applicable` when nothing is attending the machine. An
attended baseline with no model loaded measures 0.17–0.47 cores, so both bounds sit clear
of normal operation.

**The two ranges are different, and both are real.** The model serves 8k–64k. A machine
someone is using is admissible to **56k**; 64k is unattended-only. Every cell above
completed its request, so nothing but the desktop column distinguishes the last row.

The compositor **stalls** rather than saturates — the failing cell's peak sits below every
passing cell's minimum, so the populations do not overlap — and it is flat from the first
sample of the cell, which points at allocation time rather than at ingest.

**That ceiling is conditional on what else is running, and the condition is doing more work
than the number.** It was measured with browsers closed, at a 5.81 GB apparatus. A normal
working set on this machine measures **20.32 GB** — a browser alone was 5.84 GB — and the
model is 18.10 GB at 8k rising only to 20.30 GB at 64k:

| apparatus | 8k | 32k | 64k | Q3_K_M weights alone |
|---|---|---|---|---|
| 5.81 GB, browsers closed | 23.9 GB | 24.9 GB | 26.1 GB | 19.6 GB |
| 20.32 GB, normal use | 38.4 GB | 39.5 GB | 40.6 GB | 34.1 GB |

**Nothing fits alongside a normal working set — not the smallest context, not the smallest
quant in 0004's ladder.** And context is the wrong lever for it: an eightfold cut in context
saves 2.2 GB, where closing a browser saves 5.8. Running this model locally means clearing
the desk first; that is a property of a 27B on 32 GB, not a tuning problem.

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
| Thinking at `xhigh` | 3.06× completion tokens, 1.67× wall, on tier-1 tasks — **this is xhigh vs off, not thinking vs off**; see the reasoning-effort section |

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

## Reasoning effort is a four-point axis, and its default is the bad end

Qwen3.8 ships a `reasoning_effort` dial — `xhigh` (default), `medium`, `low` — separate from
`enable_thinking`. `llama-server` honours it both as a top-level field and inside
`chat_template_kwargs`, with identical results; the harness sends the top-level form the model
card documents and records the level on every row.

**The default is `xhigh`, and it is not a sane default for agentic work.** Qwen's own notes
scope it to "complex tasks demanding thorough analysis", and on this suite it does not
terminate: three tier-1 tasks burned their entire budget on reasoning and never wrote an
answer, one of them producing 27,234 characters of it at four times the original cap. Every
thinking-mode number recorded before 2026-08-18 was taken at `xhigh` whether it says so or not.

The four settings on the three tasks that move (three passes each, 32k/q8_0):

| task | off | low | medium | xhigh |
|---|---|---|---|---|
| `patch-contradiction-rounding` | 3/3 | 1/3 | 2/3 | 0/3, never terminated |
| `patch-off-by-one` | 3/3 | 3/3 | 3/3 | 2/3, one non-termination |
| `toolcall-constraint-readonly` † | 2/3 | 3/3 | 3/3 | 3/3 |

† re-measured after its fixture was fixed; the earlier figures scored the model for refusing
to patch a file it had not been shown. Its `xhigh` runs complete once the cap is 4096 rather
than 1024, so that task's non-termination really was a budget problem — unlike the
contradiction task, where raising the cap only bought a longer spiral.

> **Superseded for the toggle.** A clean comparison exists — each mode at its own
> model-card sampling, in `docs/data/2026-08-18-m2max-32gb-0005-toggle.jsonl`. Read that for
> anything crossing the toggle. The block below stands only for what does not cross it.
>
> **Void as a thinking comparison.** Every row in this section was taken at the server's
> default sampling, identical in both modes. The vision requires each mode to run at its own
> recommended sampling — `temp 1.0 / top_p 0.95 / top_k 20` on, `temp 0.7 / top_p 0.80 /
> top_k 20 / presence_penalty 1.5` off — and calls a fixed-sampling comparison of the toggle
> void. So the `off` column cannot be compared with the others here. What *is* clean is the
> comparison **among** `low`, `medium` and `xhigh`, which share both the toggle and the
> sampling. 0005 owns the redo; `cmd/eval` now refuses to set the toggle without sampling.

**Reasoning made this model worse where it moved at all — except where it did the opposite.**
On the contradicted-specification task, off passes every time and thinking fails 3 of 6, always
on the same assertion. On the read-only tool-choice task the direction reverses: off reaches for
the forbidden `edit_file` once in three, and every run with reasoning on takes the constraint.
Two tasks, two directions, three runs a cell — enough to kill "more thinking is better" as a
working assumption, not enough to replace it with a rule. On the contradicted-specification
task, off passes every time and thinking fails 3 of 6 — always on the same assertion, the model
resolving the contradiction case by case rather than picking one rule. `medium` beating `low`
is within the noise of three runs and is not a ranking. What the data supports is that more
reasoning is not a free upgrade here, which is the opposite of what 0005 was built to assume.

## The harness

`cmd/eval` drives a fixed suite against a running server and writes one JSON row per run,
into `docs/data/`.

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

Budgets are set from the fast end (`reasoning off`) plus headroom, so a task hitting its
budget means something changed, not that the number was tight. One pass over the 14 tasks
costs **5.4 minutes** with reasoning off; the same pass at `xhigh` cost 16.8 and did not
finish six of its runs.

**`reasoning_effort` is passed to the server verbatim rather than checked against a list.**
Qwen3.8 takes `low`/`medium`/`xhigh` and the next model will take something else; a harness
that hardcodes one vendor's vocabulary has to be edited before it can measure anything new.
What is guaranteed instead is that whatever was sent appears on every row.

## Sampling and thinking, settled

Each mode at its own model-card sampling; 9 ranking tasks, 3 passes, 32k/q8_0.

| setting | pass | tool-call valid | wall |
|---|---|---|---|
| **off · `temp 0.7 / top_p 0.80 / top_k 20 / presence_penalty 1.5`** | **25/27** | 11/12 | **5.0 min** |
| off · greedy | 21/27 | 9/12 | 4.4 min |
| off · temp 0.3 | 23/27 | 10/12 | 4.3 min |
| off · temp 1.0 | 24/27 | 11/12 | 8.4 min |
| off · top_p 0.95 | 25/27 | 11/12 | 4.4 min |
| **on/low · `temp 1.0 / top_p 0.95 / top_k 20`** | **25/27** | **12/12** | 18.6 min |
| on/low · greedy | 24/27 | 12/12 | 16.4 min |
| on/low · temp 0.7 | 24/27 | 12/12 | 18.0 min |
| on/medium | 24/27 | 12/12 | 22.4 min |

**Nothing beats the model card, in either mode.** Colder is monotonically worse without
thinking — 21, 23, 25 as temperature rises to 0.7 — and `top_p` does nothing at all. **Greedy
is the worst setting measured**, which is worth knowing because it is the tempting choice for
a reproducible sweep.

**Reasoning does not earn its cost at either profile.** It is 4× the wall clock for 25/27
against 25/27. The one metric favouring it is tool-call validity, 12/12 against 11/12 — a
single failure. Eight of nine tasks are 3/3 in every cell, so the claim is *no gain detectable
on this suite*, not *no gain exists*.

So both profiles take the same setting, which is a result and not an assumption:

| profile | thinking | sampling |
|---|---|---|
| attended | **off** | `temp 0.7 / top_p 0.80 / top_k 20 / presence_penalty 1.5` |
| unattended | **off** | as above — nothing was found for the 4× to buy |

The modes fail *differently* on the one task that moves, which the pass rate hides: off fails
the plain-arithmetic cases both readings of a contradictory spec agree on, while thinking gets
the arithmetic right and then resolves the contradiction case by case. Any later claim that a
mode is better must say at what.

## Tool-call adherence is not a formatting problem

Across **150 recorded tool-call runs**: 136 pass, 10 wrong-but-valid, 2 from a fixture since
fixed, 2 truncated at a cap. **Zero unparseable, zero schema-invalid.** The entire remaining
deficit is choosing the wrong tool under a stated constraint.

Constrained decoding is therefore not pursued: a grammar makes malformed calls impossible and
we have none, while it cannot fix tool choice and would cost sampling speed. The template also
asks for an XML call form — `<function=name>` with `<parameter=key>` — so a JSON-schema
constraint would fight it rather than help.

## Which tier-1 tasks carry signal

Measured at `off` and `xhigh` across the full 14-task suite, and at all four levels for the
three below that moved.

| tasks | verdict |
|---|---|
| `patch-contradiction-rounding` | **discriminates** — the only task with a genuine, repeatable split |
| `toolcall-constraint-readonly` | **weakly discriminates, in the opposite direction** — off 2/3, every reasoning level 3/3. Its earlier failures were a fixture flaw (the model refusing to patch a file it had not been shown), fixed by supplying the source in the prompt and re-measured |
| `patch-nil-check`, `patch-off-by-one`, `patch-sibling-merge`, `patch-sibling-splitpath`, `retrieval-2000/8000/16000`, `retrieval-distractor-2000/8000`, `toolcall-constraint-unknown-path`, `toolcall-edit-file`, `toolcall-read-file` | **flat** — 3/3 at every setting measured. They are the floor check that catches a config broken outright, and they cost seconds; they cannot rank anything |

A summary over the whole suite is therefore diluted by twelve columns that cannot move. Read
the discriminating subset, and keep the rest as the floor check they are.

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
- **`cmd && run || echo "skipped"` reports a failure as a skip.** The lint target used that
  shape to tolerate a missing binary, so when golangci-lint was present *and found issues* the
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
  `iogpu.wired_limit_mb` is a separate budget, defaulting to about 75% of physical memory,
  and Metal buffers come out of it. The symptom is a glitching desktop, not a slow model —
  and the model reports success throughout, because it is the process that got the memory.
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
