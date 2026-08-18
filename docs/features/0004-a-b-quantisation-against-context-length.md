---
id: 0004
title: A/B quantisation against context length
status: Draft
created: 2026-08-17
shipped:
check:
checked:
review:
needs: 0014
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

Quants run upward as well as downward, but **which ones are worth downloading is 0014's
answer, not an assumption here.** Q5_K_M and Q6_K were added on the strength of 0003's
"headroom exists" finding, and that finding came from a ladder blind to the GPU wired
ceiling — the constraint that actually makes the desktop unusable at 64k. The question is
still whether headroom is better spent on weight quality or context length; what changed
is that the headroom may not be spendable while someone is using the machine.

The full ladder is 74.5 GB of downloads at a measured ~3 MB/s, most of a day, so the
order matters. Projected against the only two points known — 19.13 GB resident is fine
and 20.27 GB is not — Q5_K_M lands near 22 GB at 32k and Q6_K near 25 GB, which would put
both outside the attended profile entirely. **That is an estimate, and estimates about
this machine have been wrong repeatedly**, which is exactly why 0014 runs first and this
feature downloads nothing until it has.

Q3_K_M is the exception worth fetching regardless: it is the only quant that buys
materially more context, and at 13.8 GB it is comfortably under any plausible threshold.

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

**0014 has answered the gate, and it answered with a wired budget rather than a context.**
The desktop survives 22.18 GB of wired memory and fails at 22.29, and the failure happens at
**load**, not under load — so a config's admissibility is a property of what it allocates and
can be judged from a load plus a minute of sampling. A 13-minute verdict is now a 90-second
one, which is what makes this grid affordable to filter before downloading anything.

Against that budget the projection this feature was gated on holds, and now has a measurement
under it: Q4_K_M at 32k reaches 21.75 GB wired, leaving roughly 0.5 GB of admissible room.
Q5_K_M is about 3 GB heavier and Q6_K about 6, so both land past the line for attended use
before quality is measured at all. **Download order should therefore be Q3_K_M first**, and
the larger quants only as unattended candidates — which is also where `iogpu.wired_limit_mb`
becomes worth raising, since the question there is whether the cap is what stops them loading
rather than whether the desktop survives.

The attended ceiling of 57,344 also carries a condition that matters more than the number: it
was measured with browsers closed, at a 5.81 GB apparatus. At a normal 20 GB working set no
quant in this ladder fits at any context. Cells are admissible against a cleared desk, and the
results file should say so.

## Tasks

- [ ] The grid is defined in a committed file, with infeasible cells marked and the reason recorded
- [ ] The two decision rules — attended and unattended — are written down with their thresholds and justification before any run
- [ ] The harness distinguishes truncation failures from reasoning failures and reports them separately
- [ ] Every feasible cell is run 3× and appended to the results file, with free memory and swap recorded per run so a contaminated cell is identifiable rather than silently averaged in
- [ ] Cells whose cold ingest exceeds the attended threshold are marked unattended-only rather than dropped, since that is a real and useful answer
- [ ] The quant ladder is chosen from 0014's measured desktop threshold, and every quant excluded by it is recorded as unattended-only rather than dropped — the distinction 0010 also needs
- [ ] Only the quants that survive that filter are downloaded, since the full ladder is 74.5 GB at ~3 MB/s
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
- 2026-08-18 — gated on 0014. The larger quants entered this grid because 0003 reported
  memory headroom, and 0003 could not see the GPU wired ceiling that makes 64k unusable
  with a desktop running. Projected forward, Q5_K_M and Q6_K would sit outside the
  attended profile before quality is measured at all, so downloading 74.5 GB to find that
  out would be spending a day to learn something a threshold measurement answers first.
  Q3_K_M stays justified on its own terms.
- 2026-08-18 — 0014 shipped and this feature is ungated. What it inherits: an admissible wired
  budget of ~22.2 GB rather than a context length, a failure that occurs at load so cells can
  be screened in 90 seconds instead of 13 minutes, and a projection — Q5_K_M and Q6_K past the
  line for attended use — that now rests on a measurement instead of an extrapolation. The
  `iogpu.wired_limit_mb` lever comes here too, as an unattended-profile question about what
  stops a larger quant loading.
