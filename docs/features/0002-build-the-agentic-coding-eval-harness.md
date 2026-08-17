---
id: 0002
title: Build the agentic coding eval harness
status: Draft
created: 2026-08-17
shipped:
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

Two things the original plan conflated, kept separate here because they change at
different rates:

- **The harness** is the agent loop — Claude Code, Pi, Hermes, OpenCode. Chosen, not written (0010).
- **The scorer** is this feature: a Go binary that runs fixed tasks through a harness
  against an endpoint and records what happened.

The scorer drives a harness through a **thin adapter**: how to invoke it non-interactively
with a task prompt in a given directory, and how to tell when it has stopped. Adapters for the
candidates 0010 compares, written together because an interface with one implementation is
an abstraction inventing itself — and Claude Code's is the one that must exist first, since
it is 0010's baseline and 0008's flow. Everything else — task
definitions, scoring, metrics, results — is shared.

Each task runs in a **scratch git checkout**, and passing means the repo's own tests pass
afterwards. Deterministic checks are what make this an instrument; an LLM judge would add
a second model's noise to every number this project rests on.

Metrics come from the cheapest honest source rather than from new code:

| Metric | Source |
|---|---|
| Task success | The repo's tests, in the scratch checkout |
| Tool-call validity | Harness logs plus the server's request log |
| Generation / prompt-processing tok/s | `llama-server` timings; `llama-bench` for isolated throughput |
| Peak memory, context high-water | Sampled during the run (0003 owns the envelope) |

Results append as one row per (config, harness, task, run) to a committed file, so sweeps
accumulate and nothing is recomputed to be compared. Runs repeat **3×** — sampling makes a
single run unfalsifiable — and the scorer reports spread, not just the mean.

Six to ten tasks, fixed and small. A suite too slow to re-run stops being re-run, and every
sweep multiplies it by the config count.

## Tasks

- [ ] Each candidate harness is confirmed to run non-interactively against the local endpoint, with the exact invocation recorded — or the blocker is written up before any scorer code exists
- [ ] A Go binary runs one task through one harness in a scratch checkout and exits non-zero on failure
- [ ] Adapters for the 0010 candidates satisfy the same interface, each proven on the same task
- [ ] Six to ten fixed tasks exist as committed fixtures, each with a deterministic pass check that runs the repo's own tests
- [ ] All five metrics are recorded per run and appended to a results file in a stable schema
- [ ] `make eval CONFIG=<label> HARNESS=<name>` runs the suite 3× and prints a per-metric summary with spread
- [ ] The scorer scores 0001's baseline end to end, and that first row is committed as the reference

## Open questions

- Does every candidate expose a scriptable non-interactive mode? This is the assumption
  the whole design rests on, hence task one. If one cannot be driven headlessly it leaves
  0010 by default, and that should be recorded as the reason rather than presented as a
  quality result.
- Is a fixed task suite representative of real agentic work? Leaning: **no, not fully** —
  which is why 0008 exists and why divergence between live use and this suite is a finding
  the suite has to answer for.

## Log

- 2026-08-17 — stack settled as Go, per `docs/INBOX.md`; question answered and cleared.
- 2026-08-17 — rescoped: the harness is chosen (Pi/Hermes), not written. Only the scorer
  is built here, which is a fraction of the original scope.
