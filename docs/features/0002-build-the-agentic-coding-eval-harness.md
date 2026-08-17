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
related: 0001
---

## Problem

"Which config is best for coding" cannot be answered by vibes or by perplexity, and
every A/B in this project (0004, 0005, 0006, 0007) plus the entire fine-tune decision
(0009) is scored by this harness. Without it they are all opinion, and the vision's
"measurement before tuning" constraint is unenforceable.

## Non-goals

- **Not a general LLM benchmark.** No MMLU, no leaderboard reproduction. This measures
  one thing: does a local model drive an agentic coding loop on this hardware.
- **Not a coding agent.** It drives an endpoint with a fixed tool schema; it does not
  implement planning or editing. The vision rules that out.
- **No comparison runs.** Building the instrument is this feature; running the sweeps
  is 0004 onward. Shipping this means it can score a config, not that anything is scored.
- **No cloud baseline.** Scoring against a hosted model would send code off the machine.

## Design

The harness runs **N fixed tasks** against an OpenAI-compatible endpoint, each a real
repo-shaped job — locate a symbol and edit it, add a test, fix a failing build — run in
a scratch git checkout so success is checked by running the repo's own tests, not by
judging prose. Deterministic scoring is what makes it a measuring instrument; an
LLM-judge would add a second model's noise to every number.

Five metrics per config, because they trade against each other and a single score hides it:

| Metric | Why it decides something |
|---|---|
| Task success rate | The only quality number that matters here |
| Tool-call validity rate | Malformed JSON is the dominant local-model failure, and it is invisible in success rate alone when a retry saves it |
| Generation tok/s | Whether the loop is usable interactively |
| Prompt-processing tok/s | Dominates agentic latency — every turn re-reads a growing context |
| Peak memory + context high-water | Whether the config survives a long session (0003 owns the ceiling) |

Results are written as one row per (config, task, run) to a committed results file, so
sweeps append and nothing is recomputed to be compared. Runs are repeated **3×** —
sampling makes a single run unfalsifiable — and the harness reports spread, not just mean.

Task count is deliberately small and fixed. A suite too slow to re-run is one that stops
being re-run, and every sweep multiplies it by the config count.

## Tasks

- [ ] A runner takes an endpoint URL and a config label, executes one task in a scratch checkout, and exits non-zero on failure
- [ ] Tool-call transport: a fixed tool schema, plus per-turn recording of whether the model's call parsed and validated
- [ ] Six to ten fixed tasks exist as committed fixtures, each with a deterministic pass check that runs the repo's own tests
- [ ] The runner records all five metrics per run and appends rows to a results file in a stable schema
- [ ] A `make eval CONFIG=<label>` runs the suite 3× and prints a per-metric summary with spread
- [ ] The harness scores 0001's baseline end to end, and that first row is committed as the reference

## Open questions

- Nothing here is settled until the stack is: see `docs/INBOX.md`. Leaning **Go** —
  `kit` is Go, `kit help go-checklist` exists, and a single static binary avoids a Python
  environment competing with the model for memory during a run.
- How many repeats is enough? Leaning 3 to start, revisited once the observed spread on
  the baseline is known — if it is wide, the suite is measuring noise.

## Log
