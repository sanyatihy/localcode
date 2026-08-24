---
id: 0033
title: Refuse a resume that would change the session budget in silence
status: Shipped
created: 2026-08-24
shipped: 2026-08-24
needs: 0032
---

## Problem

The budget a session gets depends on what the server serves, and nothing says when that
changes. A chain resumed against the default config was handed a 10,240-token ceiling where
its previous sessions had 22,528, and the run continued for two starved sessions before the
difference was noticed by reading `session.json` by hand. Every number needed to catch it is
already written there.

## Non-goals

- No refusal to run. A smaller window is a legitimate choice and sometimes the only one the
  machine has; what is wrong is making it silently, mid-chain.
- No memory of what the config was. The served context is the authority — this compares
  against what the last session actually got, not against what a file claims.

## Design

At the start of a resumed chain the supervisor reads the previous session's `session.json`
and compares its limits to the ones it is about to use. A difference is narrated and
recorded, naming both numbers and the endpoint, because a chain whose sessions were budgeted
differently is one whose session rows cannot be compared.

Narrated and recorded, not refused: the operator who lowered the ceiling on purpose should
not have to argue with the tool, and the one who did it by accident needs to be told once,
loudly, at the point where it is still cheap to fix.

`-sessions` and `-calls` are compared the same way and for the same reason — all three are
what a session row means.

## Tasks

- [x] a resume whose session budget differs from the last session's says so before it runs
- [x] the difference is recorded where the chain's ending is, so it survives the terminal

## Log
- 2026-08-24 — `-sessions` is not compared. The design groups it with the ceiling and the
  call budget, but a session's row does not depend on the chain's bound, and raising that
  bound is exactly what a resume after a bounded stop is for — comparing it would warn on
  the commonest resume there is.
