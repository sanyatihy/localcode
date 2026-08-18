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
- 2026-08-17 — merging 0001 renamed the eval knob to `LABEL`: both branches had defined
  `CONFIG`, one as the serve config path and one as the results label, and either winning
  silently would mislabel results or launch the wrong server.
- 2026-08-17 — **the fixtures were starving thinking mode**: 8 of 8 failures under
  thinking=on hit their token cap exactly, none a quality failure. Retrieval allowed 64
  tokens, which thinking spends before answering. Caps became 512/1024/2048, identical in
  both modes, and the scorer separates a capped answer from a wrong one.
- 2026-08-17 — **first clean matrix, and it is a ceiling effect rather than a verdict.** 42
  runs, zero truncations, every task 3/3 in both modes, so the suite did not discriminate.
  Decisive for attended — 1.67x slower at no measured gain — and no evidence at all for
  unattended, whose case rests on long-horizon reasoning this suite does not contain. 0013
  is the fix. Rows in `docs/data/2026-08-17-m2max-32gb-tier1-matrix.jsonl`.
- 2026-08-18 — tier 2 lands. `Driver` is declared in the consumer and kept to two methods
  all three harnesses agree on. **All three drove patch-nil-check to a pass**, scored by the
  unseen test: pi 35.0 s, opencode 142.5 s, hermes 302.7 s — same model, same 64k server, an
  8.6x spread.
- 2026-08-18 — both open questions settled. Every candidate exposes a scriptable mode
  (`pi -p`, `opencode run`, `hermes -z`), so nobody leaves 0010 by default. And a fixed suite
  does **not** represent real agentic work — measured, not suspected: 21/21 in both modes
  means it cannot rank configs, which is 0013's subject.
