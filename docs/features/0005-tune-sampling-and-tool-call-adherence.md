---
id: 0005
title: Sweep thinking and sampling per profile
status: Draft
created: 2026-08-17
shipped:
check:
checked:
review:
needs: 0002
related: 0004
---

## Problem

The dominant failure mode of a local model in an agentic loop is not weak reasoning but
a malformed tool call: a dropped brace, an invented field, prose wrapped around JSON.
Each one costs a retry or kills the turn, and sampling settings plus the chat template
drive that rate far more than most config tuning admits. This is cheap to fix and
expensive to leave broken.

## Non-goals

- **No quantisation or context work.** 0004's axis; this runs on whatever baseline exists.
- **No fine-tuning.** Out of scope project-wide. If format adherence survives every
  sampling and grammar option here, the remaining levers are a different model (0007) or
  a harness that recovers from bad calls (0010) — never the weights.
- **No prompt engineering of the task suite.** Changing tasks to suit the model would
  invalidate every other feature's numbers.

## Design

**Thinking mode is the primary axis, and it is the one most likely to split the two
profiles.** Measured on the live baseline: the same tool call cost 70 completion tokens
with thinking and 28 without, and both produced a correct call with valid JSON. At 7-14
tok/s that difference is tens of seconds per turn, which attended use pays directly and
unattended use barely notices — while the reasoning it buys is exactly what an unattended
run needs, since nobody is there to catch a wrong step. So the hypothesis this feature
tests is **thinking off for attended, thinking on for unattended**, and it is a hypothesis,
not a finding: the 28-vs-70 probe showed a trivial call surviving, not hard reasoning.

**Thinking and sampling are coupled and must move together.** The model card gives
`temperature 1.0 / top_p 0.95 / top_k 20` for thinking mode and `temperature 0.7 /
top_p 0.80 / top_k 20 / presence_penalty 1.5` for non-thinking. A sweep that holds
temperature fixed across the toggle is measuring the pair. Each mode is therefore swept
at its own recommended values, with greedy as a floor case for both.

Three levers, measured in increasing order of intrusiveness so the cheapest sufficient
one wins:

1. **Thinking and sampling, per profile** — the two modes at their own recommended
   settings, then temperature and top-p varied within each.
2. **Chat template correctness** — whether `llama-server` applies the model's own template
   and tool-call format exactly. A wrong template looks like a bad model and is the first
   thing to rule out, so it is verified before any sampling is swept.
3. **Constrained decoding** — llama.cpp's GBNF grammar / JSON-schema constraint, which can
   make malformed calls structurally impossible. It has a cost: a grammar can force
   syntactically valid but semantically wrong calls, and it slows sampling. So it is
   measured, not assumed better.

Tool-call validity rate is the primary metric here; task success is the guard against
optimising format at the expense of reasoning — a config that emits perfect JSON and
solves nothing has lost. **Every result is reported per profile**, and "the same setting
won both" is a finding worth stating explicitly rather than an assumption to start from.

Because template correctness can invalidate everything downstream of it, it is task one.

## Tasks

- [ ] The chat template and tool-call format `llama-server` applies are verified against the model's own definition, and any mismatch is fixed
- [ ] Thinking on and thinking off are each swept at their own model-card sampling defaults, never at a shared temperature, and scored on tool-call validity and task success
- [ ] A sampling sweep within each mode — temperature and top-p around the defaults, plus greedy as a floor — is scored the same way
- [ ] Results are reported per profile, and the recommendation says which setting won attended and which won unattended
- [ ] Constrained decoding via GBNF/JSON-schema is measured against the best unconstrained config, including its speed cost
- [ ] Failures are classified by cause (unparseable, schema-invalid, wrong-but-valid) so the remaining deficit is named rather than counted
- [ ] The chosen sampling config is committed, and `docs/TECH.md` records the validity rate before and after

## Open questions

- Adopt constrained decoding as the default even if it only ties on validity? Leaning
  **no** — a tie means the grammar is buying nothing and costing speed, and it hides
  model deficits that 0007 and 0010 need to see.

## Log

## Log

- 2026-08-17 — retitled and rescoped around thinking mode after live probes on the
  baseline: 70 completion tokens with thinking vs 28 without for an identical, correct
  tool call. Also recorded the coupling — each mode has its own recommended sampling, so
  the toggle cannot be swept at a fixed temperature.
