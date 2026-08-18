---
id: 0013
title: Add discriminating tasks to the tier-1 suite
status: Draft
created: 2026-08-17
shipped:
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
- [ ] The extended suite runs both thinking modes 3x, and the spread is reported — a flat result means the tasks failed and are rewritten before anything is concluded from them
- [ ] `docs/TECH.md` records which tasks discriminate and which do not, so later features know which subset carries signal

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

- Should non-discriminating tasks be retired once identified? Leaning **no** — they are the
  floor check that catches a config broken outright, and they cost seconds. But they should
  be *labelled*, so a summary is not diluted by columns that cannot move.
- How hard is too hard? Leaning: a task no config passes after the first calibration run is
  cut, because it consumes runtime in every future sweep and reports nothing.

## Log
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
