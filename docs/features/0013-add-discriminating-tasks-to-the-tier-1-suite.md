---
id: 0013
title: Add discriminating tasks to the tier-1 suite
status: Shipped
created: 2026-08-17
shipped: 2026-08-18
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
- 2026-08-18 — **every off-vs-on conclusion in this feature is void.** All 114 rows here
  were collected at the server's default sampling, identical in both modes, where thinking
  carries its own recommended pair — so they measure the pair rather than the toggle. What
  survives is everything that does not cross the toggle: `xhigh` not terminating, the
  comparison among low/medium/xhigh, and the flatness of the twelve floor-check tasks. 0005
  redoes the toggle; `cmd/eval` now refuses to set it without sampling.
- 2026-08-18 — seven tasks authored, taking the suite from 7 to 14. Retrieval needed a
  scorer change as well as fixtures, since distractors only discriminate if naming a decoy
  fails.
- 2026-08-18 — **first calibration run: the suite reads flat, and by this feature's own rule
  that means the tasks are wrong.** One genuine failure in 78 clean runs; every other task
  3/3 in both modes, the five new ones included. The traps are real — the fixture self-tests
  prove the tempting answers fail — so this is a fact about the model, not broken fixtures.
- 2026-08-18 — three tasks were unresolved rather than flat: their caps decided the outcome
  before quality could. Raised to 8192 for patch and 4096 for tool-call, the new fixtures
  having inherited 2048/1024 from the older, easier tasks.
- 2026-08-18 — **the cap diagnosis was wrong for `patch-contradiction-rounding`, and the
  mechanism behind it was `reasoning_effort`.** Qwen3.8's dial defaults to `xhigh` and this
  harness never set it, so every thinking-mode number recorded here was taken there. It is
  `xhigh` that fails to terminate — at `low` and `medium` the task converges in ~200 s and
  fails the trap 3 of 6, against 3/3 with reasoning off. The task discriminates after all.
- 2026-08-18 — which makes the first calibration's verdict wrong too: it compared the two
  ends and never sampled between them. The rewrite is still owed for eleven tasks, not for
  the one that was carrying signal all along.
- 2026-08-18 — `toolcall-constraint-readonly`'s failures were the fixture's fault: the model
  declined to patch a file it had not been shown, having been given no read tool. The source
  is now in the prompt rather than adding a read tool, which would have made reading first
  legitimate where the task checks the *first* call. Re-measured at all four levels, it
  discriminates weakly and in the opposite direction.
- 2026-08-18 — **shipped at six of six, with the rewrite moved to BACKLOG rather than left
  open.** "Rewrite eleven tasks until this model fails them" is open-ended work with no
  guarantee of converging, and the suite is already good enough for what depends on it: a
  floor check that catches a broken config in 5.4 minutes, plus one task that genuinely
  ranks. 0010 does not need tier-1 to rank at all, and 0004 and 0005 read the labelled
  subset.
