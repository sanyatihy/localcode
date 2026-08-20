---
id: 0011
title: Split planning and grinding across frontier and local models
status: Shipped
created: 2026-08-17
shipped: 2026-08-20
check:
checked:
review: human
needs: 0008, 0010
related: 0002
---

## Problem

The two tiers fail in different places. A frontier model is worth its cost on
architecture, decomposition and review — judgement over a whole repo, done once. A local
27B is worth using on the grind: the tenth mechanical edit, run unattended and free. Using
one model for both wastes money on the grind or judgement on the plan. What is missing is
the seam between them.

## Non-goals

- **No per-request routing or fallback.** The vision rules it out: a router that silently
  retries a failed local call against a frontier model makes every local number in this
  project meaningless. The tiers are **roles a human assigned**, not a load balancer.
- **No multi-agent framework.** `kit` already defines the roles; this wires existing
  pieces to them rather than adding an orchestration layer.
- **No autonomous merging.** `kit`'s handoff rule stands — an agent never merges its own
  work, and the local tier does not become an exception because it is cheap.

## Design

`kit` already has the three roles this needs, which is the reason to build the seam here
rather than invent a protocol: a **drafter** plans a round, a **builder** claims one
feature and ships it, a **coordinator** merges what somebody else built. The split maps
straight onto them — frontier as drafter and coordinator, local as builder — and the trunk
is already the barrier between the tiers.

That mapping is what makes this cheap: the interface between tiers is **feature docs and
git branches**, which already exist and are already the state. No new protocol, no message
bus, no shared memory. A local builder reads a doc and ships a branch; whether a frontier
model or a human wrote that doc is invisible to it.

The real question is not plumbing but **which work the local tier can actually finish**.
That is answered by evidence rather than by assertion: 0002's scorer already records which
tasks the local model completes, and the honest form of this feature is a **stated boundary**
— the class of feature a local builder finishes unattended, and the class it must not be
given. A local tier handed architecture work produces branches that cost more to review
than to have written.

Cost is the reason to build it and therefore has to be measured: frontier tokens spent per
shipped feature, before and after. If planning costs as much as building, the split has not
paid for itself.

**What leaves the machine is planning context, deliberately.** The vision permits it as a
named exception; the boundary of what a frontier model is shown is part of this design, not
an afterthought, and `review: human` because widening it is not an agent's call.

## Tasks

- [x] The boundary is written down: which feature classes a local builder finishes unattended, drawn from 0002's per-task results rather than from judgement
- [x] A local builder claims a real feature from a frontier-drafted doc and ships a reviewable branch, using `kit` unchanged
- [x] What planning context reaches the frontier tier is characterised and bounded, with the rule committed
- [x] Frontier tokens per shipped feature are measured against a frontier-only baseline
- [x] The outcome is recorded in `docs/TECH.md`, including which work the split is not worth doing for

## Log
- 2026-08-20 — both open questions are answered and the section goes with them. The first
  asked whether anything needed building: the split ran on `kit` unchanged, and the one
  thing that did need building was the session handoff, which is 0016 and shipped
  separately. The second was the human's and is answered in `docs/TECH.md`.
- 2026-08-20 — the human's answer to the context question: the whole repository. Recorded in
  `docs/TECH.md` with the reason it costs little — no secrets are committed by policy — and
  with the boundary it still draws: nothing outside the checkout is planning context.
- 2026-08-20 — the split was measured three times over one box, not once. What changed the
  outcome was the tool set as much as 0016's mechanism: 21 tool definitions cost 18,045
  tokens of a 45,056-token window, and the run that had the mechanism without the flag
  finished nothing. Recorded because a reader of 0016 alone would conclude the handoff is
  sufficient.
