---
id: 0002
title: Build the agentic coding eval harness
status: Shipped
created: 2026-08-17
shipped: 2026-08-18
check:
checked:
review:
needs:
related: 0010, 0001
---

## Problem

Every A/B here — quant against context, sampling, llama.cpp against MLX, model against
model, Pi against Hermes — needs one instrument that says which side won. Without it
they are all opinion, and the vision's "measurement before tuning" constraint is
unenforceable. What is missing is not an agent: Pi and Hermes already are agents. What
is missing is the thing that runs fixed tasks through one of them and records numbers.

## Non-goals

- **Not a coding agent, and not a custom harness.** The single most expensive mistake
  available here. Pi (`earendil-works/pi`) and Hermes (`NousResearch/hermes-agent`) are
  model-agnostic and already speak OpenAI-compatible endpoints. This feature drives one;
  it does not reimplement one. If neither can be driven headlessly, that is a finding for
  `## Open questions`, not a licence to start writing an agent loop.
- **Not a general LLM benchmark.** No MMLU, no leaderboard reproduction.
- **No comparison runs.** Building the instrument is this feature. Running sweeps with it
  is 0004 onward; shipping this means it *can* score a config, not that anything is scored.
- **No reimplementation of throughput benchmarking.** `llama-bench` and the server's own
  timings already measure tok/s. Wrapping beats rewriting.
- **No cloud baseline.** Scoring against a hosted model would send code off the machine.

## Design

**Harness and scorer are separate**, because they change at different rates: the harness is
the agent loop (chosen, not written — 0010), the scorer is this feature. The scorer drives a
harness through a **thin adapter** — invoke it non-interactively with a prompt in a
directory, tell when it stopped — written for all of 0010's candidates at once, since an
interface with one implementation is an abstraction inventing itself.

**Two tiers, because they cost differently and answer differently.** Tier 1 talks straight
to the endpoint: seconds per task, enough to sweep a toggle matrix, and independent of the
harness question, so it comes first. Tier 2 drives a real harness through a multi-turn task
in a scratch git checkout — the only way to see coherence and recovery — at minutes per run.

**Passing means the repo's own tests pass.** Deterministic checks are what make this an
instrument; an LLM judge would add a second model's noise to every number this project rests
on.

| Metric | Source |
|---|---|
| Task success | The repo's tests, in the scratch checkout |
| Tool-call validity | Harness logs plus the server's request log |
| Generation / prompt-processing tok/s | `llama-server` timings; `llama-bench` for isolated throughput |
| Peak memory, context high-water | Sampled during the run (0003 owns the envelope) |

Results append one row per (config, harness, task, run) so sweeps accumulate and nothing is
recomputed to be compared. Runs repeat **3×** — a single run is unfalsifiable — and the
scorer reports spread. Six to ten fixed tasks: a suite too slow to re-run stops being
re-run, and every sweep multiplies it by the config count.

## Tasks

**Tier 1 — direct to the endpoint, no harness involved:**

- [x] A Go binary sends one fixed task straight to the endpoint, applies a deterministic check, and exits non-zero on failure
- [x] Six to ten tier-1 tasks exist as committed fixtures: tool-call correctness against an expected call, patch tasks checked by compiling and running the result, and retrieval probes at increasing context depth
- [x] Metrics are recorded per run — success, tool-call validity, tok/s generated, tok/s prompt, cached-token share — and appended to a results file in a stable schema
- [x] `make eval LABEL=<label>` runs the suite N× against a named config and prints a per-metric summary with spread
- [x] The request-level toggle matrix runs end to end: thinking on and off, each at its own model-card sampling defaults, reported per profile

**Tier 2 — through a real harness, for multi-turn behaviour tier 1 cannot see:**

- [x] Each candidate harness is confirmed to run non-interactively against the local endpoint, with the exact provider configuration recorded — or the blocker is written up
- [x] An adapter interface drives at least two harnesses through the same tier-2 task in a scratch checkout, pass checked by the repo's own tests

## Log

- 2026-08-17 — rescoped: the harness is chosen (Pi/Hermes), not written. Only the scorer is
  built here.
- 2026-08-17 — a tier-1 task must have exactly one defensible action. `toolcall-edit-file`
  scored a style preference — the model read before editing, which is reasonable — and would
  have failed every config identically. Fixed by inlining the file content.
- 2026-08-17 — fixture Go files are `.go.txt`: left as `.go` they sit inside this module, so
  `go test ./...` compiled the deliberately-broken fixtures and the gate went red.
- 2026-08-17 — each results row records what the server reported serving — `n_ctx`, model
  file — not just the label a human typed. A label is a claim; a restart that did not take
  would otherwise attribute one config's numbers to another.
- 2026-08-17 — spread is min-max, not a standard deviation: three passes is already a
  multi-hour job and a deviation over three samples claims precision the data lacks. Runs are
  sequential because the server has one slot.
- 2026-08-17 — merging 0001 renamed the eval knob to `LABEL`: both branches defined `CONFIG`,
  one as the serve config path and one as the results label, and either silently winning
  would mislabel results or launch the wrong server. `check` now runs unit tests and smoke.
- 2026-08-17 — **the fixtures were starving thinking mode**: 8 of 8 failures under
  thinking=on hit their token cap exactly, none a quality failure. Retrieval allowed 64
  tokens, which thinking spends before answering. Unfixed, the matrix would have reported
  thinking hurting quality when the gap was a budget set while testing with thinking off.
  Caps are 512/1024/2048, identical in both modes, and `fail_truncated_at_cap` separates a
  capped answer from a wrong one.
- 2026-08-17 — **first clean matrix: 42 runs, zero truncations, every task 3/3 in both
  modes.** Thinking costs 3.06x completion tokens (1089 → 3333) and 1.67x wall (384 s →
  641 s) and buys nothing this suite can detect.

  | task | off: pass / tok / s | on: pass / tok / s |
  |---|---|---|
  | patch-nil-check | 3/3 · 137 · 15.0 | 3/3 · 221 · 23.9 |
  | patch-off-by-one | 3/3 · 100 · 10.7 | 3/3 · 438 · 46.6 |
  | retrieval-2000 | 3/3 · 12 · 7.7 | 3/3 · 73 · 14.8 |
  | retrieval-8000 | 3/3 · 12 · 26.9 | 3/3 · 93 · 37.4 |
  | retrieval-16000 | 3/3 · 13 · 55.4 | 3/3 · 70 · 64.4 |
  | toolcall-read-file | 3/3 · 28 · 4.4 | 3/3 · 64 · 8.6 |
  | toolcall-edit-file | 3/3 · 61 · 8.0 | 3/3 · 152 · 18.0 |

- 2026-08-17 — **that result is a ceiling effect and must not be read as a quality verdict.**
  Both modes scored 21/21, so the suite did not discriminate. Decisive for attended — 1.67x
  slower at no measured gain — and no evidence at all for unattended, whose case for thinking
  is long-horizon reasoning this suite does not contain. 0013 is the fix.
- 2026-08-17 — harness configuration is repo-local per harness: Pi an extension registering a
  provider, OpenCode a `provider` block using `@ai-sdk/openai-compatible`, Hermes a top-level
  `model:` block with `provider: custom` (the `providers.<name>` map reverse-engineered from
  its source is real but not how a local endpoint is configured, and produced a connection
  error while never opening a connection).
- 2026-08-17 — **Hermes refuses any context window under 64,000 tokens**, checked before any
  request. Pi and OpenCode run at 32k and Hermes cannot, so a like-for-like comparison must
  put all three at 64k — where 0003 measured a cold ingest of 13.1 minutes against 5.4 at
  32k. 0010 inherits that cost.
- 2026-08-18 — tier 2 lands. `Driver` is declared in the consumer and kept to two methods,
  all three harnesses agree on. **All three drove patch-nil-check to a pass**, scored by the
  unseen test: pi 35.0 s, opencode 142.5 s, hermes 302.7 s — same model, same 64k server, an
  8.6x spread.
- 2026-08-18 — two bugs visible only through the adapter. Relative config paths resolved
  against the scratch checkout because `cmd.Dir` is the workdir, so pi hunted for its
  extension under /tmp; paths are absolute at validation now. And `cmd.Dir` does not update
  `PWD`, which Go replaces wholesale when `Env` is set, so a tool resolving its project from
  `PWD` looks in the launching directory — OpenCode reported that as "Unexpected server
  error".
- 2026-08-18 — `RunTier2` refuses a fixture that passes before the harness runs, and without
  calling the driver: the tier-1 lesson encoded.
- 2026-08-18 — both open questions settled. Every candidate exposes a scriptable mode
  (`pi -p`, `opencode run`, `hermes -z`), so nobody leaves 0010 by default. And a fixed suite
  does **not** represent real agentic work — measured, not suspected: 21/21 in both modes
  means it cannot rank configs, which is 0013's subject.
