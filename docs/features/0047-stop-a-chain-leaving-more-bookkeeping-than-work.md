---
id: 0047
title: Stop a chain leaving more bookkeeping than work
status: Draft
created: 2026-08-27
shipped:
needs:
---

## Problem

The turn bound is keyed by the context the gate measured, so every turn opens a counter
file named after its peak and nothing ever closes one. Measured on this machine: one chain
left **1,247** of them, and **2,584 of the 3,213 files** under
`~/.local/state/localcode` are counters. Four fifths of everything the driver has ever
written is bookkeeping.

## Non-goals

- **Not changing what the bound is.** Four calls a turn, decided against a context reading
  that does not move while the turn runs, is what turns the reserve into a bound rather
  than a hope — measured, a five-call turn carried the context 1,960 tokens past a ceiling
  it had been under.
- **Not a retention policy for chains.** How long a chain's sessions are worth keeping is a
  separate question, and it is in `## Open questions` because only the developer can answer
  it.

## Design

**A file per turn is a cache-buster, not a record.** The peak is in the filename so that a
new reading means a new counter, which is a correct way to say "this is a different turn"
and a wrong way to store it: the key is unbounded, so the store is too. One file holding
the peak it counts for, rewritten when the peak changes, says the same thing in O(1) — and
it is the same atomic append the counters already rely on, plus a header nobody contends
on, because within one turn every hook reads the same peak.

**It also closes a collision the current key hides.** Two turns that report an identical
peak share a counter today, so the second is refused every call it makes. That is unlikely
and it is not impossible, and a scheme that cannot express "a new turn at the same reading"
cannot be made to.

**A directory of 1,247 entries is read, not just stored.** `chain.SessionIDIn` scans a
session directory to recover the id the gate counted against, and `piRecorder.Transcript`
scans the same directory for its session file. Both walk every counter to find one name.

**What is kept is what a chain is read back from.** The handoff, `session.json`,
`sessions.jsonl`, `chain.json`, the transcript and the calls counter are the record
`localcode account` and `localcode sessions` read; the per-turn counter is scaffolding that
outlived its turn. Only the scaffolding goes.

## Tasks

- [ ] One turn's bound is held without a file per turn, and the batch counter a session
      leaves is bounded by its sessions rather than by its turns
- [ ] Two turns that report the same context reading each get their own bound
- [ ] A chain that has already run keeps working, and its old counters do not change what
      a new session may spend

## Open questions

- **Whether `localcode` should forget old chains.** The state directory is 18 MB and grows
  with every chain ever run; nothing prunes it and `localcode sessions` lists all of them.
  Leaning: a `localcode forget` that takes an age or an id, rather than anything automatic
  — a chain is the only record of what a session cost, and a driver that deletes evidence
  on a timer is the wrong default for a project whose output is measurements.

## Log
