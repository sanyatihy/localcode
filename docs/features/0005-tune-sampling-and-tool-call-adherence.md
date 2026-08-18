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

**Thinking is not a binary and its default is the bad end.** Qwen3.8 exposes
`reasoning_effort` — `xhigh` (default), `medium`, `low` — beside `enable_thinking`, so this
feature's axis has four points, not two. 0013 measured all four on the tasks that move and
found reasoning made the model *worse* where it moved at all: the contradicted-specification
task passes 3/3 with reasoning off and fails 3 of 6 with it on, always on the same assertion.
`xhigh`, the default, did not terminate at all.

That inverts this feature's working assumption. The question is not how much thinking to buy
but **whether thinking earns its cost on this workload at any level**, and `off` is a serious
candidate rather than the cheap baseline the sweep was going to beat. Any run that does not
record its effort level is uninterpretable — see `docs/TECH.md`, where every figure taken
before 2026-08-18 is `xhigh` whether it says so or not.

**This feature now owns a correction, not just a sweep.** 0013 collected 114 rows comparing
thinking on against off at the server's default sampling, which the vision declares void: each
mode has its own recommended pair, so a fixed-sampling comparison measures the pair. Every
toggle conclusion from 0013 is therefore unsupported and is redone here — and the redo is the
cheap part, because `cmd/eval` now refuses to set `-thinking` without either
`-sampling-profile thinking|nonthinking` or explicit sampling flags.

**Judge it on tool-call validity and cost, not on task success.** Twelve of the fourteen tier-1
tasks are 3/3 at every setting measured, so a sweep scored on pass rate will return another flat
result — 0013 already ran that experiment. The metrics with room to move are tool-call validity
rate, tokens per completed task, and wall clock, all of which the scorer already records.

## Tasks

- [x] The chat template and tool-call format `llama-server` applies are verified against the model's own definition, and any mismatch is fixed — **no mismatch; nothing to fix**
- [x] Thinking on and thinking off are each swept at their own model-card sampling defaults, never at a shared temperature, and scored on tool-call validity and task success
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
- 2026-08-18 — **the feature's hypothesis is falsified: thinking earns its cost at neither
  profile.** 81 runs, each mode at its own model-card sampling. Pass rate 25/27 off, 25/27 at
  `low`, 24/27 at `medium`; wall clock 5.0 min against 18.6 and 22.4. The design predicted off
  for attended and *on* for unattended, on the argument that an unattended run needs the care
  nobody is there to supply — but there is no quality for unattended to buy with the 4×. The
  one metric favouring thinking is tool-call validity, 12/12 against 11/12, which is a single
  failure and a hint rather than a result. Eight of the nine tasks are 3/3 in every cell, so
  the supportable claim is *no gain detectable on this suite*, not *no gain exists*.
- 2026-08-18 — the two modes fail differently on the one task that moves, which the pass rate
  hides: off fails `TestRoundHalfAwayFromHalves`, the plain-arithmetic cases both readings of
  the spec agree on, while thinking fails `TestRoundHalfPicksOneRuleAndKeepsIt`, the
  consistency trap. Reasoning fixes the arithmetic and then talks itself into an inconsistent
  resolution. Any later claim that one mode is "better" has to say at what.
- 2026-08-18 — template verified, no mismatch, and the verification changed what the other
  boxes must account for. `reasoning_effort` is **a system message the template injects**,
  not a sampling parameter: at `low` it renders "Keep your thinking brief and focused", at
  `xhigh` "think carefully through the task, validate key assumptions, consider plausible
  alternatives". It is prepended to the task's own system prompt rather than replacing it,
  and with `enable_thinking:false` it is not injected at all — so the two knobs compose. With
  nothing set the template injects the `xhigh` text, which puts the default beyond inference.
  The tool-call format the template asks for is **XML-style** — `<function=name>` with
  `<parameter=key>` blocks — not JSON inside `<tool_call>`, and `llama-server` converts it to
  OpenAI `tool_calls` with JSON arguments correctly. That is a constraint on the constrained-
  decoding box: a GBNF grammar has to target the XML form, and a JSON-schema constraint would
  fight the template rather than help it.
- 2026-08-17 — retitled and rescoped around thinking mode after live probes: 70 completion
  tokens with thinking against 28 without, for an identical correct tool call. Each mode has
  its own model-card sampling, so the toggle cannot be swept at a fixed temperature.
- 2026-08-18 — inherited from 0013: `reasoning_effort` is a real axis with four points, its
  default is `xhigh`, and `xhigh` does not terminate on some tier-1 tasks. Reasoning lowered
  the score wherever it changed anything, so `off` is a candidate to beat rather than a floor.
