---
id: 0004
title: A/B quantisation against context length
status: Draft
created: 2026-08-17
shipped:
check:
checked:
review:
needs: 0002, 0003
related: 0001
---

## Problem

Weight quantisation and context length compete for the same 32 GB: a bigger quant buys
per-token quality and takes it straight out of the context budget. Nobody knows which
side of that trade wins for agentic coding on this machine, and it is the single highest-
leverage config decision in the project — it sets the default every other feature builds on.

## Non-goals

- **No sampling parameters.** A separate axis, and 0005 owns it. Sweeping both at once
  multiplies the grid and confounds the result.
- **No other models or runtimes.** 0006 and 0007, deliberately after this, so they inherit
  a settled quant-and-context default instead of sweeping a third axis.
- **No new metrics.** Whatever 0002 measures is what decides this. Adding a metric here to
  explain a result is how a sweep becomes a story.

## Design

A grid of **weight quant × context length**, run on 0002's harness, bounded above by
0003's working ceiling so no cell swaps. Quants: Q3_K_M, Q4_K_M, UD-Q4_K_XL, Q5_K_M —
spanning the range that can fit at a usable context, including one unsloth dynamic quant
to test whether it beats the plain one at equal size. Contexts: the ladder rungs that fit
each quant. Cells that cannot fit are recorded as **infeasible, not run** — an unfilled
cell is a result and must be legible as one.

The decision rule is written **before** the runs, because a rule chosen after seeing the
numbers is a preference — and there are **two rules, one per profile**:

- **Attended:** highest task success rate among configs whose generation speed clears the
  interactive threshold. Speed is a gate here, because a human is waiting.
- **Unattended:** highest task success rate, full stop, subject only to fitting in memory.
  There is no latency gate — a config that is twice as slow and more correct wins outright.

Both thresholds are stated in the results file as numbers, not adjectives, and come from
0003 and 0001's observations rather than being invented here. The same config winning both
is a possible and welcome outcome; assuming it in advance is what this split prevents.

The expected tension is that the biggest quant that fits will force a context so small
that agentic tasks fail by truncation rather than by reasoning. That is a real outcome
and must be reported as such — truncation failures are counted separately from wrong
answers, or the sweep will blame the model for a budget problem.

## Tasks

- [ ] The grid is defined in a committed file, with infeasible cells marked and the reason recorded
- [ ] The two decision rules — attended and unattended — are written down with their thresholds and justification before any run
- [ ] The harness distinguishes truncation failures from reasoning failures and reports them separately
- [ ] Every feasible cell is run 3× and appended to the results file
- [ ] A winning config is identified per profile by the pre-written rules, with runner-up and margin for each, and it is stated plainly whether one config won both
- [ ] The winner becomes the default config from 0001 — or two named configs if the profiles diverge — and `docs/TECH.md` records the table and both decisions

## Open questions

- Should Q3_K_M be included given the quality cliff below 4 bits? Leaning **yes** — it is
  the only quant buying a materially larger context, and this is the one place that trade
  can be measured instead of assumed.

## Log

## Log

- 2026-08-17 — split the decision rule per profile. Attended gates on latency because a
  human waits; unattended does not, so a slower, more correct config can win it.
