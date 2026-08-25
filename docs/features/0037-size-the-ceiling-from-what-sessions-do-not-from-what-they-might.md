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
measured and taken back separately. A third was found by measuring and is the largest: the
harness holds back a quarter of what it is told, and that check can be turned off.

**The harness's check is a duplicate of a decision already made here.** It exists to stop a
session sending a prompt the server will refuse, which is what the gate does — from a
transcript reading, against a ceiling, with a handoff on the other side of the denial. Left
on, it costs a quarter of the context and cannot be reasoned about, since the fraction is
undocumented and moves with the version. Off, the bound is ours: the window is what the
server will take, which is the served context less what a reply may generate.

**What replaces it is an error, not a check.** llama-server names both numbers — `request
(49509 tokens) exceeds the available context size (49152 tokens)` — so a session that
overruns says so precisely, and the driver can record it rather than reporting a session
that failed for no stated reason. That is the failure the reserve exists to prevent, so
seeing one at all is the signal that the reserve is wrong.

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

## Tasks

- [x] the overshoot past the ceiling is measured across every chain this repository has run
- [x] the reserve is derived from that measurement, and refuses when the arithmetic leaves a
      session no room
- [x] the boundary at which the harness refuses a prompt is measured against a server told
      its whole context
- [x] the window is the largest prompt the harness will send, which takes back the
      reservation subtracted twice and gives up the quarter it never had
- [x] the harness's own context check is off, and the window is what the server will take
- [x] a session that overruns the server is recorded as that, and the chain carries on

## Log

- 2026-08-25 — the overshoot is 5,486 tokens at the worst over 8 chains and 69 sessions, and
  the growth past the reading the gate decided on is 7,881. The two reserve terms are not
  equally wrong: the results reach 72% of the bound the gate creates, the turns a third of
  twice the output reservation.
- 2026-08-25 — four turns land after that reading, not the two the reserve was reasoned
  from: the batch bound permits four calls and the handoff grace three writes. The largest
  single one generated 1,305 tokens against the 639 the problem records.
- 2026-08-25 — the margin the open question asks for is two, and the measurement supports it:
  a reserve of a quarter of the window plus 6,144 stands at 2.1x the worst growth recorded at
  both windows served. The turn term is twice the worst any chain generated, not a fraction
  of what the harness would allow.
- 2026-08-25 — the ceiling rises from 22,528 to 24,576 at the shipped window, and a declared
  context under 20,480 is now refused: the reserve is a constant where it used to follow the
  output reservation, so the smallest windows lose what the largest gain.
- 2026-08-25 — the harness refuses at three-quarters of the declared context less its
  reservation, not at the whole of it: 34,008 tokens sent and 34,258 refused against a
  declared 49,152, and the same fraction within 4% at three other declarations.
- 2026-08-25 — the reservation is subtracted twice and taking it back is safe, because the
  largest prompt sent plus the whole reservation is 38,104 of the 49,152 served. That was
  the question the box asked; the larger answer is that the window was never the wall.
- 2026-08-25 — the last box is rewritten. It asked for the window to take back what is
  reserved twice, and the measurement says that is 4,096 tokens the window may have and a
  quarter of it the window never had. Taking back the one without giving up the other would
  raise the ceiling to 27,648 against a wall at 34,134 and a growth past it measured at
  7,881 — a session budgeted to land exactly on `Prompt is too long`.
- 2026-08-25 — the shipped ceiling goes from 22,528 to 19,200 and the call budget from 51 to
  41. Both are smaller than what shipped and both are the first ones under the wall: a
  session at the old ceiling that grew the 7,881 tokens the worst one did reached 30,409
  against a limit of about 31,200 it had never been told about.
- 2026-08-25 — the smallest context that still admits a session is now about 28,672 declared.
  Three quarters of a window is what the harness will send, and a quarter of what is left is
  a turn's results, so a small context runs out of working room before it runs out of window.
- 2026-08-25 — the feature reopens. `CLAUDE_CODE_DISABLE_UNKNOWN_MODEL_WINDOW_ENFORCEMENT`
  turns the harness's check off, which makes three quarters a limit this project chose to
  accept rather than one that binds. Measured: with it set, a 44,508-token prompt is sent
  against a declared 49,152 where 34,258 was refused with it unset.
- 2026-08-25 — what the check was protecting against is now llama-server's own 400, which
  names the request and the context size. A session cannot recover from it, so the gate is
  the only thing preventing it and the driver's job is to say when it happened.
- 2026-08-25 — the ceiling is 27,648 and the budget 66 calls at the shipped context, against
  22,528 and 51 before any of this. Three quarters of that gain is the harness's check being
  off and the rest is the two reserves.
- 2026-08-25 — the smallest context that still admits a session is about 24,576 served, where
  it was 16,384 before. The reserve is a constant where it used to follow the output
  reservation, so the small end loses what the large end gains.
