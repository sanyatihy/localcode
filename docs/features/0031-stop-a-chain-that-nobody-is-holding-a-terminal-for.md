---
id: 0031
title: Stop a chain that nobody is holding a terminal for
status: Draft
created: 2026-08-24
shipped:
needs: 0029
---

## Problem

A chain is stopped by interrupting it, and the code says the session shares the process
group so the interrupt reaches both. That holds for a terminal, where the signal goes to the
foreground group. It does not hold for a chain started in the background and signalled by
pid: the supervisor exits, the session it was waiting on keeps running against the endpoint,
and stopping it takes finding and killing the child by hand. Measured on a real run, an
orphaned session survived its supervisor and had to be matched by its `--add-dir` argument.

## Non-goals

- No process supervision beyond the chain's own children. A session that spawns something
  which outlives it is the sandbox's business.
- No change to what an interrupt means. It still finishes the running session where it can,
  because a session killed mid-edit leaves work a handoff cannot describe.

## Design

The supervisor puts each session in its own process group and forwards the interrupt to it,
rather than relying on a group it does not own. Forwarding, not killing: the session is
given the same signal it would get from a keyboard, so its own shutdown path runs and the
handoff still lands.

A second interrupt kills, because the reason to press it twice is that the first did not
work — and a supervisor that cannot be stopped is worse than a session that dies mid-edit.

The ending records `interrupted` through 0029, so a chain stopped this way is
distinguishable afterwards from one that hit its bound.

## Tasks

- [x] an interrupt sent to a background chain reaches the session it is waiting on
- [ ] a second interrupt ends the run without waiting

## Log
