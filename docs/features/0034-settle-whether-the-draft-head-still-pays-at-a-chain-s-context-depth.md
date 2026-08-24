---
id: 0034
title: Settle whether the draft head still pays at a chain's context depth
status: Shipped
created: 2026-08-24
shipped: 2026-08-24
needs:
---

## Problem

The driver config trades context for decode speed: the MTP draft head is a measured
1.26–1.40x and it refuses to load at 49,152, so the profile serves 32,768 and a session gets
a 10,240-token ceiling. That trade was settled on twenty independent bugs. On a 25-session
chain it looks inverted — at the context depths a chain actually reaches, sessions with the
draft head decoded at 6.93 tok/s and sessions without it at 6.94, while the larger ceiling
turned eight sessions that committed nothing into three that shipped a feature.

## Non-goals

- No change to the editor profile. Its first request measured 36,309 tokens of preamble,
  which is a different shape, and 0025 chose for it deliberately.
- No new speedup. This settles which of two configurations already in hand the driver
  should serve, and nothing else.

## Design

The comparison is one task run both ways, because that is the only form the existing
evidence is missing: the numbers above come from different work at different depths, which
is a signal and not a measurement. The task has to be one the small ceiling can finish, or
the result is the ceiling rather than the decode rate.

Decode rate is read per session from the accounting 0027 already produces, against the peak
context of the session that produced it — the confound to control is depth, since decode
falls with it whatever is drafting. What the chain costs end to end is the number that
decides, because a chain that ships in three sessions has paid three preambles and one that
stalls in eight has paid eight.

`docs/TECH.md` carries the 32,768 side of this trade today. Whichever way it lands, the
table is what changes.

## Tasks

- [x] one task runs to completion on both configurations, at comparable context depth
- [x] the same question is read off the chains this repository has already run on real
      source, because the fixture the first box needs is one the small ceiling can finish
- [x] the driver serves whichever config the run settles on, and TECH.md says why

## Log

- 2026-08-24 — the run does not reproduce the signal the feature was drafted on. At matched
  depth the head decodes 9.33 tok/s against 6.75, which is the 1.38x 0017 measured, so the
  6.93-against-6.94 reading came from work at different depths rather than from the head
  having stopped paying.
- 2026-08-24 — a third arm was added: `config/agent.env` pinned to the driver's 10,240
  ceiling. Two configs at their own ceilings differ in depth as well as in mechanism, and
  the design named depth as the confound to control without saying how.
- 2026-08-24 — `scripts/chainrun.sh` gained a `CEILING` knob, which is what pinning the
  third arm needed.
- 2026-08-24 — the config does not change. The run settles on what the driver already
  serves, so this box is the table in `docs/TECH.md` and nothing in `config/`.
- 2026-08-24 — a third box, between the two. The first box's design requires "a task the
  small ceiling can finish", which pre-selects work the small ceiling is good at: 0032
  measured a call on the generated fixture at 298 tokens against 683 on real source, so the
  fixture fits nineteen calls in the room a repository fits eight in. The fixture run cannot
  settle the question it was built for, and the chains already run can.
- 2026-08-24 — the two readings disagree and both stand. At one depth on the fixture the
  draft head is 1.38x and the smaller context finishes first; on real source every session
  at that ceiling ends on the ceiling, the preamble rises to 46% of ingest, and the
  advantage falls to nothing per tool call.
- 2026-08-24 — the default moves to `config/agent.env`, which reverses the third box's first
  answer. What decides it is nine tool calls a session against thirty-one, not the wall
  clock, which is within 3% per call either way.
