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
