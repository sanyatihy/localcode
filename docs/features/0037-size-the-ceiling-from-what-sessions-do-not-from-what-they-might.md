---
id: 0037
title: Size the ceiling from what sessions do, not from what they might
status: Draft
created: 2026-08-25
shipped:
needs:
---

## Problem

The ceiling is what binds a chain: 19 of the 24 sessions of a real run ended on it, three on
the call budget and two on their own work. It is 22,528 of a 49,152-token context, and the
reserves that make it that small have never been tested by a session — the worst overshoot
past the ceiling in that run was 5,486 tokens against the 18,432 held back for it, and
12,946 tokens of window went unused at the session that came closest.

## Non-goals

- No change to what the reserve is for. What lands after the gate's last reading has to fit,
  and a ceiling that does not leave room for it is a session that hits the wall while being
  protected from it.
- No clamping. A window whose arithmetic does not work is still refused, because a session
  that discovers it spends a cold ingest to say `Prompt is too long`.
- No re-decision of the draft head. A larger ceiling changes what 0034 settled, and that is
  a comparison to redo afterwards rather than a change to make here.

## Design

Two reserves are taken out of the window and they are wrong in different ways, so they are
measured and taken back separately.

**The output reservation is subtracted twice.** `declaredFromServer` sets
`CLAUDE_CODE_MAX_CONTEXT_TOKENS` to what is served less the reservation, and `NewLimits`
takes the reservation off that again to get the window. The second is the harness's own
behaviour, bisected against a declared 12,288; the first applies the same reasoning to the
same number. Whether it is redundant is one bisection against a server told its whole
context, and it is worth 3,072 tokens of ceiling.

**The reserve is sized for a session that has never run.** It holds a quarter of the window
for four tool results and twice the output reservation for the two turns that follow the
gate's last reading. The second term is the one furthest from what was measured: it reserves
8,192 tokens for turns that generated about 200 tokens each, and the handoff turn — the
largest — ran 639. The first is nearer its bound, since a `Read` may return the whole result
cap. So the terms are judged apart, against the overshoot every chain here has already
recorded.

**What must still hold is a property, not a number.** The ceiling plus the worst overshoot
must stay under the window with margin, and the margin is what the measurement sets. A
reserve derived from an observed distribution needs a floor the distribution cannot see: the
four maximum-sized results a turn is permitted are a bound the gate itself creates, and no
run so far has spent it.

## Open questions

- What margin over the worst observed overshoot is enough. Two is a guess; the distribution
  across every chain in `docs/data/` is what should answer it, and it may say the tail is too
  short to extrapolate from at all.

## Tasks

- [x] the overshoot past the ceiling is measured across every chain this repository has run
- [ ] the reserve is derived from that measurement, and refuses when the arithmetic leaves a
      session no room
- [ ] the boundary at which the harness refuses a prompt is measured against a server told
      its whole context
- [ ] the window takes back whatever that measurement shows is reserved twice

## Log

- 2026-08-25 — the overshoot is 5,486 tokens at the worst over 8 chains and 69 sessions, and
  the growth past the reading the gate decided on is 7,881. The two reserve terms are not
  equally wrong: the results reach 72% of the bound the gate creates, the turns a third of
  twice the output reservation.
- 2026-08-25 — four turns land after that reading, not the two the reserve was reasoned
  from: the batch bound permits four calls and the handoff grace three writes. The largest
  single one generated 1,305 tokens against the 639 the problem records.
