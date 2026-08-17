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

**The suite is two tiers, because they cost different amounts and answer different
questions.** Tier 1 talks straight to the endpoint over HTTP: no harness, no agent loop,
seconds per task. It measures single-turn quality — does it call the right tool with valid
arguments, does its patch compile and pass, does it still retrieve correctly at depth — and
that is enough to sweep a toggle matrix. Tier 2 drives a real harness through a multi-turn
task, which is the only way to see coherence and recovery, and costs minutes per run.

Tier 1 comes first because it is what the serving sweeps actually need, and because it is
independent of the harness question. Tier 2 tasks run in a **scratch git checkout**, and
passing means the repo's own tests pass Deterministic checks are what make this an instrument; an LLM judge would add
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

**Tier 1 — direct to the endpoint, no harness involved:**

- [x] A Go binary sends one fixed task straight to the endpoint, applies a deterministic check, and exits non-zero on failure
- [x] Six to ten tier-1 tasks exist as committed fixtures: tool-call correctness against an expected call, patch tasks checked by compiling and running the result, and retrieval probes at increasing context depth
- [x] Metrics are recorded per run — success, tool-call validity, tok/s generated, tok/s prompt, cached-token share — and appended to a results file in a stable schema
- [x] `make eval LABEL=<label>` runs the suite N× against a named config and prints a per-metric summary with spread
- [ ] The request-level toggle matrix runs end to end: thinking on and off, each at its own model-card sampling defaults, reported per profile

**Tier 2 — through a real harness, for multi-turn behaviour tier 1 cannot see:**

- [ ] Each candidate harness is confirmed to run non-interactively against the local endpoint, with the exact provider configuration recorded — or the blocker is written up
- [ ] An adapter interface drives at least two harnesses through the same tier-2 task in a scratch checkout, pass checked by the repo's own tests

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
- 2026-08-17 — tier-1 scorer runs. Verified against the live baseline both ways and,
  more importantly, against two negative controls: expecting the wrong tool and an
  unsatisfiable argument each fail with a distinct outcome and exit 1. Outcomes are
  classified rather than boolean (no call / wrong tool / invalid JSON / wrong args /
  server error) because 0005 needs to know *how* a config fails, and because a server
  error must never be scored as model quality. Checker failure paths are covered by an
  offline test so the instrument does not depend on a server behaving.
- 2026-08-17 — seven tier-1 fixtures land and all run end to end. Both patch fixtures were
  confirmed to fail their own unseen tests before any model saw them; a fixture that passes
  out of the box would score every config correct and mean nothing.
- 2026-08-17 — the first suite run caught a defect in the instrument rather than the model.
  toolcall-edit-file failed with the model calling read_file when told to make an edit — but
  reading before editing is defensible agent behaviour, so the fixture was scoring a style
  preference as a wrong answer, and would have failed every config identically. Rewritten to
  include the file content inline, making a read provably unnecessary; the model then calls
  edit_file correctly. The general rule this teaches: a tier-1 task must have exactly one
  defensible action, or it is measuring the fixture author, not the model.
- 2026-08-17 — retrieval passes at 2k, 8k and 16k depth on the q8_0 KV baseline, so the
  quantised cache is not visibly costing recall at the context this config serves. That is a
  single sentinel per depth, not a rate — it establishes the probe works, and 0004 is what
  turns it into evidence about KV types.
- 2026-08-17 — fixture Go files renamed to .go.txt. Left as .go they sit inside this
  module, so `go test ./...` compiled and ran the deliberately-broken fixtures and the
  repo gate was red for exactly the reason the fixtures are correct. The scratch module
  writes them back under real .go names.
- 2026-08-17 — results are JSONL, one row per (config, task, repeat). Each row records
  what the server reported serving — n_ctx and model file — not only the config label a
  human typed. When the sweep starts varying server-level flags, a label is a claim, and
  a row carrying the served n_ctx is what stops one config's numbers being attributed to
  another after a restart that did not take.
- 2026-08-17 — suite mode, `make eval CONFIG=… N=…`, and a reporter grouping by config
  and thinking mode. Spread is min-max rather than a standard deviation: three passes is
  already a multi-hour job here, and a deviation over three samples claims precision the
  data does not have. Runs are sequential because the server has one slot — concurrent
  requests would queue and every timing would measure the queue. A transport failure
  records and continues rather than aborting, since losing hours of sweep to one dropped
  connection is worse than a gap in the data.
- 2026-08-17 — merging 0001 collided on the Makefile in two ways that mattered more than
  the textual conflict. Both branches defined `CONFIG`: 0001 as the path to the env file
  scripts/serve.sh consumes, documented in docs/TECH.md, and 0002 as the label written
  into every results row. Either silently taking the other's value would have mislabelled
  results or launched the wrong server, so the eval knob is renamed `LABEL` and 0001's
  shipped meaning stands. Both also defined `check`, one as unit tests and one as the
  live smoke; `check` is the ship gate in AGENTS.md, so it now runs both, with `test`
  and `smoke` available separately.
- 2026-08-17 — audited the Go against `kit help go-checklist`, which I had not read before
  writing it. Real defects, not style: runPatch used a hand-rolled timer that killed the
  process but left its reader goroutine blocked and ignored the caller's context, now
  `exec.CommandContext`; both commands did their work in `main` with scattered
  `os.Exit`, bypassing deferred cleanup, now `run() error` with one exit point and
  documented codes; `time.Now()` was uninjected, now a package-level `Now` a test can
  set; `go.mod` had no toolchain pin. `make check` gated on tests alone — it now runs
  fmt, vet and `-race`, and the live smoke moved to `make smoke` because
  `kit init --stack go` wired CI to `make check`, and CI has no server or weights.
  Tests moved to the public boundary (Client.Run against an httptest double), which also
  reaches failure paths a real model cannot be asked to produce on demand.
