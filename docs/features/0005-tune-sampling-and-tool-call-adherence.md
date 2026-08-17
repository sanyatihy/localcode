---
id: 0005
title: Tune sampling and tool-call adherence
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

Three levers, measured in increasing order of intrusiveness so the cheapest sufficient
one wins:

1. **Sampling** — temperature, top-p, top-k, min-p, repetition penalty. Qwen models ship
   recommended values; those are the starting point, not the answer, and greedy decoding
   is included as the floor case.
2. **Chat template correctness** — whether `llama-server` applies the model's own template
   and tool-call format exactly. A wrong template looks like a bad model and is the first
   thing to rule out, so it is verified before any sampling is swept.
3. **Constrained decoding** — llama.cpp's GBNF grammar / JSON-schema constraint, which can
   make malformed calls structurally impossible. It has a cost: a grammar can force
   syntactically valid but semantically wrong calls, and it slows sampling. So it is
   measured, not assumed better.

Tool-call validity rate is the primary metric here; task success is the guard against
optimising format at the expense of reasoning — a config that emits perfect JSON and
solves nothing has lost.

Because template correctness can invalidate everything downstream of it, it is task one.

## Tasks

- [ ] The chat template and tool-call format `llama-server` applies are verified against the model's own definition, and any mismatch is fixed
- [ ] A sampling sweep — including greedy and the Qwen-recommended defaults — is scored on tool-call validity and task success
- [ ] Constrained decoding via GBNF/JSON-schema is measured against the best unconstrained config, including its speed cost
- [ ] Failures are classified by cause (unparseable, schema-invalid, wrong-but-valid) so the remaining deficit is named rather than counted
- [ ] The chosen sampling config is committed, and `docs/TECH.md` records the validity rate before and after

## Open questions

- Adopt constrained decoding as the default even if it only ties on validity? Leaning
  **no** — a tie means the grammar is buying nothing and costing speed, and it hides
  model deficits that 0007 and 0010 need to see.

## Log
