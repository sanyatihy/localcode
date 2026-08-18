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

A grid of **weight quant × context length**, run on 0002's harness. 0003 removed the
constraint this grid was designed around: nothing from 8k to 64k swaps, and 64k/q8_0 uses
20.27 GB of 32 GB, so cells are not eliminated by memory the way the original plan assumed.

Quants therefore run **upward**, not just downward: Q4_K_M, UD-Q4_K_XL, Q5_K_M and Q6_K.
Q5_K_M and Q6_K were previously excluded on a memory argument the measurement does not
support, and with roughly a third of the machine unused they are the most interesting
cells in the grid — the question this feature actually answers is whether headroom is
better spent on weight quality or on context length.

Contexts stay 8k/16k/32k. Going higher is measurable but not useful: 0003 clocked a cold
64k ingest at 13.1 minutes, which no attended profile can spend. **The grid's cost is now
denominated in ingest seconds rather than gigabytes**, and cells are dropped for taking
too long, not for failing to fit. A cell that cannot fit is still recorded as infeasible,
but that is now expected to be rare.

The decision rule is written **before** the runs, because a rule chosen after seeing the
numbers is a preference — and there are **two rules, one per profile**:

- **Attended:** highest task success rate among configs whose **cold ingest** clears the
  interactive threshold. 0003 established this is the gate that binds: 5.4 minutes at 32k,
  13.1 at 64k, and the memory gate it was paired with does not bite at any context here.
- **Unattended:** highest task success rate, full stop. No latency gate and, per 0003, no
  effective memory gate either — a config twice as slow and more correct wins outright.

The two profiles therefore sweep **different feasible sets**, not merely the same set
judged by different rules. A config can be the unattended winner and not be admissible
for attended use at all, and reporting it as "the winner" would be wrong for half the
project. Any config observed swapping is void in both, per the vision.

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
- [ ] Every feasible cell is run 3× and appended to the results file, with free memory and swap recorded per run so a contaminated cell is identifiable rather than silently averaged in
- [ ] Cells whose cold ingest exceeds the attended threshold are marked unattended-only rather than dropped, since that is a real and useful answer
- [ ] The larger quants Q5_K_M and Q6_K are measured, since 0003 showed the headroom they need exists and the plan had excluded them on an argument measurement refuted
- [ ] If a cell does meet a memory wall, the effect of raising `iogpu.wired_limit_mb` is measured there, with the exact revert command recorded — inherited from 0003, where nothing came close enough to the limit for it to mean anything
- [ ] A winning config is identified per profile by the pre-written rules, with runner-up and margin for each, and it is stated plainly whether one config won both
- [ ] The winner becomes the default config from 0001 — or two named configs if the profiles diverge — and `docs/TECH.md` records the table and both decisions

## Open questions

- Should Q3_K_M be included given the quality cliff below 4 bits? Leaning **yes** — it is
  the only quant buying a materially larger context, and this is the one place that trade
  can be measured instead of assumed.

## Log
- 2026-08-17 — the decision rule splits per profile: attended gates on latency because a
  human waits, and on 0003's working memory ceiling because the desktop is in use;
  unattended on neither. The profiles have different feasible sets, so a single "winner" is
  not always meaningful.
- 2026-08-17 — regrounded on 0003: the grid was built around a memory ceiling that does not
  exist, so quants now run upward as well as downward — Q5_K_M and Q6_K are in — and cells
  are gated on ingest seconds rather than gigabytes.
