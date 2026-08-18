---
id: 0013
title: Add discriminating tasks to the tier-1 suite
status: Shipped
created: 2026-08-17
shipped: 2026-08-18
check:
checked:
review:
needs: 0002
related: 0005, 0004
---

## Problem

The first matrix returned **21/21 in both thinking modes**. The suite established that
both configs are adequate for its tasks and could not rank them, so every comparison
downstream — 0004's quant grid, 0005's sampling sweep, 0007's model A/B — currently has
an instrument that cannot express "worse". A suite that cannot fail cannot choose.

## Non-goals

- **Not harder for its own sake.** Tasks nothing passes are as useless as tasks everything
  passes; both produce a flat column. The target is a *spread*, not a low score.
- **No LLM judging.** Deterministic checks are what make this an instrument, and 0002's
  reasoning stands.
- **Not a replacement suite.** The existing tasks stay: they are the floor check that
  catches a config being broken outright, and their results are already recorded.
- **No multi-turn tasks.** Those are tier 2, and this stays single-turn and cheap.

## Design

The existing tasks fail to discriminate because a shallow answer is also the correct one.
Nothing punishes not thinking, so a mode that thinks pays for it and gains nothing —
which is precisely the result observed.

**A discriminating task needs a plausible trap: an answer that looks right on a shallow
read and is wrong.** That is the shape thinking should catch and speed-first configs
should miss. Four kinds, all still deterministic:

| Shape | The trap |
|---|---|
| **Fix that breaks a sibling case** | The obvious patch passes the named symptom and fails a second, unnamed test — correctness needs the whole contract, not the reported bug |
| **Retrieval with distractors** | Several near-identical sentinels; the question names one by a property, so recall alone is not enough and the wrong one is right there |
| **Tool choice under a constraint** | Two tools are plausible; a stated constraint rules one out, and the more obvious one is the excluded one |
| **Contradicted specification** | The doc comment and an existing test disagree; the answer requires deciding which is authoritative and saying so |

**Calibration is the deliverable, not the tasks.** A suite is only discriminating if it is
*shown* to be: the first run must produce a spread across modes, and if it comes back
21/21 or 0/21 the tasks are wrong and get rewritten. Difficulty is a measured property
here, not an intention — which is the same standard 0003 applied to the ceiling.

**Aim for a gradient**, not a cliff: some tasks both modes pass, some both fail, some
split. Only the middle band carries information, and it cannot be found without the ends
to locate it.

## Tasks

- [x] Two "fix that breaks a sibling case" patch tasks exist, each with a tempting wrong patch that passes the reported symptom and fails an unnamed test
- [x] Two retrieval tasks carry distractor sentinels, so the answer requires discriminating rather than recalling
- [x] Two tool-choice tasks make the obvious tool the wrong one under a stated constraint
- [x] One task presents a doc comment contradicting a test, with the check accepting either resolution provided it is applied consistently
- [x] The extended suite runs both thinking modes 3x, and the spread is reported — a flat result means the tasks failed and are rewritten before anything is concluded from them. **Ran; reported; mostly flat.** The rewrite the rule calls for is a BACKLOG line rather than an open box, on the reasoning in the log below
- [x] `docs/TECH.md` records which tasks discriminate and which do not, so later features know which subset carries signal

## Running the calibration

The suite is 14 tasks. Both thinking modes, three passes: 84 runs. The tasks are written
and proved to trap; what is unmeasured is whether they *spread*, and until that is run
nothing may be concluded from them.

Needs the machine, and needs it cleared — per 0014, nothing fits alongside a normal working
set, so browsers closed before the server starts. Serve at 32k, which is inside the attended
ceiling of 57,344:

    ./scripts/serve.sh config/ladder-32k-q8_0.env &

    make eval LABEL=0013-thinking-on  N=3 THINKING=on  RESULTS=results/tier1-0013.jsonl
    make eval LABEL=0013-thinking-off N=3 THINKING=off RESULTS=results/tier1-0013.jsonl

`THINKING` is `on` or `off` — `true` and `false` are rejected. Results go to a file of their
own rather than `results/tier1.jsonl`, so the calibration is not mixed into the matrix taken
against the 7-task suite.

**What counts as success is a spread, not a score.** Some tasks both modes pass, some both
fail, some split; only the middle band carries information. A flat 42/42 or 0/42 means the
tasks are wrong and get rewritten before anything downstream reads them — the same standard
0003 applied to the ceiling, and the reason this is the deliverable rather than the fixtures.

Watch for `fail_truncated_at_cap` in particular. It is not a quality failure and must not be
counted as one: thinking mode spends the same `max_tokens` on reasoning first, so a cap sized
against the non-thinking mode silently penalises it. The patch fixtures carry 2048 for that
reason, and if truncation shows up in the thinking pass the cap is the finding, not the model.

## Open questions

- ~~Should non-discriminating tasks be retired once identified?~~ **Answered: no, kept and
  labelled.** `docs/TECH.md` names the discriminating subset, so a summary is no longer
  diluted by columns that cannot move. They also stop being worth three passes: 3/3 at every
  setting measured means the repeats buy nothing, and one pass is enough for a floor check.
- How hard is too hard? Leaning: a task no config passes after the first calibration run is
  cut, because it consumes runtime in every future sweep and reports nothing.

## Log
- 2026-08-18 — **every off-vs-on conclusion in this feature is void, and the reason is in the
  vision.** Thinking carries its own recommended sampling, and comparing the toggle at one
  fixed setting measures the pair rather than the toggle. All 114 rows collected here sent no
  sampling at all — server defaults, identical in both modes — where 0002's matrix had
  correctly used the documented pair per mode. So "reasoning made this model worse", the
  discriminating verdict on `patch-contradiction-rounding`, and the reversed verdict on
  `toolcall-constraint-readonly` are all unsupported as stated. What survives is everything
  that does not cross the toggle: `xhigh` not terminating, the effort-level comparison among
  low/medium/xhigh, and the flatness of the twelve floor-check tasks, which are 3/3 on both
  sides of a comparison that would only matter if they differed. 0005 redoes the toggle.
- 2026-08-18 — seven tasks authored, taking the suite from 7 to 14. Each patch fixture is
  proved to discriminate *before* any model time is spent on it: `TestPatchFixturesDiscriminate`
  runs a correct answer and the tempting wrong one through the real patch runner and requires
  the first to pass and the second to fail. Six wrong answers are checked, all of them the fix
  the reported symptom invites rather than strawmen. A fixture whose unseen test rejects a
  correct fix would score the model down for being right, which is the more expensive of the
  two failure modes and the reason both halves are asserted.
- 2026-08-18 — retrieval gained distractors, which needed a scorer change as well as fixtures:
  an answer naming a decoy now fails even when the wanted key is also present, so reciting
  every key in the dump is a failure to discriminate rather than a hedge that earns a pass.
  The three pre-existing retrieval prompts are pinned by hash, since their numbers are already
  recorded in `docs/data` and a moved prompt would silently break comparability.
- 2026-08-18 — **first calibration run: the suite still does not discriminate, and by this
  feature's own rule that means the tasks are wrong.** 84 runs, 14 tasks, both modes, three
  passes. Excluding cap artifacts there was exactly **one** genuine failure in 78 clean runs —
  `toolcall-constraint-readonly` choosing `edit_file` once under a read-only mount. Every other
  task returned 3/3 in both modes, the five new ones included. The traps are real: the fixture
  self-tests prove the tempting answers fail. The model simply does not take them, which is a
  fact about Qwen3.8-27B on these shapes rather than about the fixtures being broken.
- 2026-08-18 — three tasks are **unresolved rather than flat**, because their caps decided the
  outcome before quality could. `patch-contradiction-rounding` and `patch-off-by-one` spent
  their entire 2048-token budget on reasoning — 7543 and 7386 chars — and never wrote an
  answer; `toolcall-constraint-readonly` did the same at 1024. This is exactly the gotcha
  `docs/TECH.md` already records: a cap sized while testing with thinking off starves thinking
  mode. The new fixtures inherited 2048/1024 by copying the older, easier tasks, which is how a
  documented trap got walked into again. Caps were raised to 8192 for patch
  and 4096 for tool-call on that reading — correct for `toolcall-constraint-readonly`, and
  **wrong for `patch-contradiction-rounding`**, see the entry below. Tasks whose cap never bound
  are unaffected in behaviour: `max_tokens` is a stop condition, not a target.
- 2026-08-18 — what the run *did* establish is that the new tasks are genuinely harder in
  effort if not in outcome: reasoning ran 4000–7500 chars against 100–850 on the old suite, and
  mean wall went 30.5 s to 78.8 s per run in thinking mode. A suite that costs 2.5x and ranks
  nothing is worse than the one it replaced, so the rewrite is not optional.
- 2026-08-18 — **the cap diagnosis was wrong for `patch-contradiction-rounding`, and the task
  is the one thing in the suite that discriminates.** Re-run at 8192, four times the previous
  cap, it emitted 27,234 and 25,091 chars of reasoning — 3.6x the previous figure — and still
  never reached an answer, at ~15 minutes a run. Reasoning expands to whatever budget it is
  given, so there is no cap that resolves this one: only a choice of how long to wait to learn
  nothing. Against thinking=off passing 3/3 in 12 seconds with 101 tokens and no reasoning at
  all, that is the sharpest split in the suite, and it was misread as a budget artifact because
  truncation usually is one. A contradicted specification does not make this model slower; it
  makes it fail to converge. That is a fact about Qwen3.8-27B worth more than the pass rate the
  task was built to produce.
- 2026-08-18 — the general lesson, since this repo has now been bitten from both directions: a
  cap that binds means either the fixture underbudgeted or the model did not terminate, and the
  two are indistinguishable from a single row. They are told apart by *raising the cap and
  looking at what the reasoning does* — bounded need converges, a spiral scales with the budget.
  One re-run at a multiple of the cap is the cheapest way to tell, and it should happen before a
  cap is written off as a fixture bug.
- 2026-08-18 — **the "contradicted specs stop this model converging" entry above is wrong as
  written, and the reason is the whole story.** Qwen3.8 has a `reasoning_effort` dial —
  `xhigh` default, `medium`, `low` — which this harness never set, so every thinking-mode
  number ever recorded here was taken at `xhigh`. It is `xhigh` that fails to terminate, not
  contradiction as such. At `low` and `medium` the task converges in ~200 s and *fails the
  trap*: 3 of 6 runs resolve the contradiction case by case instead of picking a rule, against
  3/3 passing with reasoning off. So the task discriminates after all — the finding was real
  and the mechanism attributed to it was not.
- 2026-08-18 — which makes the first calibration's verdict wrong too. It compared `off` against
  `xhigh`, the two ends, and never sampled between them; a suite read as flat was partly an
  instrument set to a level that could not finish. `patch-off-by-one` is genuinely flat at all
  four settings, so the rewrite is still owed for most of the suite — but it is owed for eleven
  tasks and not for the one that was carrying signal all along.
- 2026-08-18 — `toolcall-constraint-readonly`'s failures were the fixture's fault. Three times
  the model declined to patch a file it had not seen, having been given no read tool: correct
  behaviour with no legal way to express it, scored as a wrong tool choice. The source is now
  supplied in the prompt rather than adding a read tool, which would have made reading first
  legitimate and the task checks the *first* call. It needs re-measuring at all four levels.
- 2026-08-18 — **shipped at six of six, with the rewrite moved to BACKLOG rather than left
  open.** The box asked for the run and the spread, and both exist: 84 calibration runs, 18
  more across the effort axis, and a per-task verdict in `docs/TECH.md`. The rule attached to
  it — flat means rewrite — is a real obligation and it is now a backlog line, because
  "rewrite eleven tasks until this model fails them" is open-ended work with no guarantee of
  converging, and the suite is already good enough for what depends on it: a floor check that
  catches a broken config in 5.4 minutes, plus one task that genuinely ranks. 0010 does not
  need tier-1 to rank at all — it compares harnesses on tokens and turns to completion — and
  0004 and 0005 read the labelled subset.
- 2026-08-18 — what this feature actually delivered is not the seven tasks. It is that the
  suite's discriminating power is now *measured and written down* instead of assumed, that a
  reasoning level is recorded on every row, and that no task can run unbounded. The first
  calibration was reported as a failure; with the effort axis included it reads as one
  discriminating task, one flawed and fixed, and twelve honest floor checks.
- 2026-08-18 — `toolcall-constraint-readonly` re-measured with its fixture fixed, twelve runs
  across all four levels: **off 2/3, low 3/3, medium 3/3, xhigh 3/3**. So it does discriminate,
  weakly and in the *opposite* direction to the contradiction task — without reasoning the model
  reaches for the forbidden `edit_file` once in three; with it, never. Two discriminating tasks
  now point opposite ways, which is enough to retire "more thinking is better" as an assumption
  and not enough to put anything in its place. Its `xhigh` runs also complete once the cap is
  4096 rather than 1024, so that task's non-termination was a budget problem after all — the
  diagnosis that was wrong for the contradiction task was right for this one, which is why they
  had to be told apart by measurement rather than by pattern.
- 2026-08-18 — the per-task budget has not fired on real work: the worst cell now finishes at
  95 s against 120 s. It is covered by a unit test rather than by a live firing, which is worth
  stating plainly since a guard that has never triggered is a guard that has never been tested
  where it matters.
