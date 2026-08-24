---
id: 0034
title: Settle whether the draft head still pays at a chain's context depth
status: Draft
created: 2026-08-24
shipped:
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

- [ ] one task runs to completion on both configurations, at comparable context depth
- [ ] the driver serves whichever config the run settles on, and TECH.md says why

## Log
