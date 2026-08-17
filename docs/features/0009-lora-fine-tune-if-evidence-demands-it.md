---
id: 0009
title: LoRA fine-tune if evidence demands it
status: Draft
created: 2026-08-17
shipped:
check: 2026-11-17
checked:
review: human
needs: 0004, 0005
related: 0006
---

## Problem

Fine-tuning is the project's most expensive option and the one most likely to be
unnecessary. It is worth doing only against a deficit that configuration provably could
not close — a specific, named, measured failure mode surviving 0004's quant/context
sweep and 0005's sampling and grammar work. Started before that evidence exists, it
spends days of compute on a problem a flag would have fixed.

## Non-goals

- **No full fine-tune.** 27B in bf16 with gradients and optimiser state needs several
  hundred GB. It is not tight on 32 GB; it is impossible, and no schedule fixes that.
- **No pre-training or continued pre-training.** Same reason, more so.
- **No teaching the model new domain knowledge.** LoRA is being considered for *behaviour*
  — format adherence, stopping, tool discipline. Facts belong in context, not in weights.
- **No training on unverified self-generated traces.** Training a model on its own
  unchecked output reinforces its errors; that is the main way this feature could make
  things worse than doing nothing.

## Design

**This feature starts by trying to cancel itself.** Task one is to state the surviving
deficit from 0005's failure classification in falsifiable terms. If none survives, the
right outcome is `kit drop 0009` with the evidence — a dropped feature with a reason is
a successful result here, not a failure.

If a deficit survives, the hardware dictates almost everything:

- **LoRA/QLoRA only**, via MLX (the practical Apple Silicon training path — hence
  `related: 0006`, which decides whether MLX is already part of the stack).
- **27B training is at the edge and may not be reachable.** A 4-bit base is ~16.4 GB
  before activations, adapter gradients and optimiser state, inside a 32 GB budget shared
  with macOS. Expect short sequences and batch size 1, and expect OOM to be a real outcome.
- **The core tension, stated up front:** the behaviour worth training is long-horizon tool
  use, and long sequences are exactly what will not fit. A LoRA trained at 2k tokens
  teaching format adherence that fails at 20k has not solved the observed problem. Sequence
  length is therefore a *result to report*, not a knob to quietly minimise until it fits.
- **Fall back to a smaller model deliberately, not accidentally.** If 27B cannot train,
  a 8–14B LoRA is comfortable — but a fine-tuned 14B is a different bet from a stock 27B,
  and 0007 already measured what that swap costs. That comparison decides it, not the
  convenience of fitting.

Data is verified trajectories only: successful runs from 0002's suite and real sessions
from 0008, each checked to be *correct* before it becomes a training example. Held-out
tasks the LoRA never trained on are what it is scored against, or the number is circular.

The bar for adoption is the same harness, unchanged: a LoRA that does not beat stock on
held-out tasks is discarded, however good its training curve looked.

## Tasks

- [ ] The surviving deficit from 0005 is stated in falsifiable terms — or this feature is dropped with that evidence recorded
- [ ] Feasibility is settled empirically: the largest model and sequence length that actually train on this machine, with OOM boundaries recorded
- [ ] A verified trajectory set is assembled from harness and live-session runs, each example checked correct, with held-out tasks reserved
- [ ] A LoRA is trained at the largest feasible sequence length, with the length and its implications reported
- [ ] The adapter is served and scored on held-out tasks against stock, and adopted only if it wins
- [ ] The outcome — including "not worth it" — is recorded in `docs/TECH.md` with the numbers

## Open questions

- If 27B cannot train but a 14B LoRA beats stock 14B while losing to stock 27B, what is
  the default? Leaning **stock 27B**, since the vision optimises the coding setup, not the
  training result — but this is the human call this feature most needs, hence `review: human`.

## Log
