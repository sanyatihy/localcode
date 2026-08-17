---
id: 0011
title: Split planning and grinding across frontier and local models
status: Draft
created: 2026-08-17
shipped:
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

- [ ] The boundary is written down: which feature classes a local builder finishes unattended, drawn from 0002's per-task results rather than from judgement
- [ ] A local builder claims a real feature from a frontier-drafted doc and ships a reviewable branch, using `kit` unchanged
- [ ] What planning context reaches the frontier tier is characterised and bounded, with the rule committed
- [ ] Frontier tokens per shipped feature are measured against a frontier-only baseline
- [ ] The outcome is recorded in `docs/TECH.md`, including which work the split is not worth doing for

## Open questions

- Does this need anything built at all, or is it a documented way of working plus `kit` as
  it stands? Leaning **strongly toward the latter** — the first task should be attempting
  it by hand, and any code proposed afterwards has to justify itself against having already
  worked without it.
- How much repo context may a frontier drafter see? A human call, hence `review: human`.

## Log
