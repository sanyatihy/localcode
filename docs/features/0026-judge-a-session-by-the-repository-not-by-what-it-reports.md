---
id: 0026
title: Judge a session by the repository, not by what it reports
status: Dropped
created: 2026-08-23
shipped:             # written by kit ship, never by hand
needs:               # feature ids that must ship first, e.g. 0002, 0003. Empty = can start now
---

<!-- `needs:` is the only field that changes what `kit next` offers, so it earns care.
     Leave it empty if this could be built today against what already exists — that is
     the common case, and several empty ones is what lets agents work in parallel.

     Judge it by whether the work could START now, not by what it touches. Clean
     boundaries hide couplings that make a feature dependent anyway:
       - a shared composition root or wiring function both features must edit
       - a shared router, registry, or dispatch table both add an entry to
       - numbered files in one sequence — migrations above all. Two features each
         adding "the next number" merge cleanly and break at runtime.
     Any of those means the second feature needs the first, however separate they look. -->


## Problem

A chain shipped a feature whose deliverable was not delivered. Its box claimed *"rule names
match doc entries"*; five taxonomy names had no rule, four rule names were absent from the
taxonomy, and the commit that claimed to reconcile them copied two files across and changed
one line — the tick. Nothing in the loop reads what a session produced: `kit ship` checks
that boxes are ticked, the chain ends when the model writes `Next: none`, and both are the
session reporting on itself.

## Non-goals

- **Not a reviewer.** Judging whether code is good needs a model and a second opinion, which
  is [0011](0011-split-planning-and-grinding-across-frontier-and-local-models.md). This asks
  only whether the repository's own gate passes, which is a question with an exit code.
- **Not a change to `kit`, and not any knowledge of it.** Ticking a box is the agent's claim
  and kit is right to take it; what is missing is a second reader, and that belongs to
  whatever drives the sessions. The gate is a command and an exit code — this must not learn
  a doc format, which is the boundary VISION draws and the one 0016's parser already
  crosses.
- **Not a bound on what a session may do.** 0023 settled that. This decides what happens
  after one ends.

## Design

**The supervisor runs the repository's own gate between sessions, and records the exit
code on the row.** The command is the repository's, not this one's: `make check`, `go test
./...`, `pytest`, whatever the project already trusts. Named by a flag and remembered with
the chain, because a chain's verification cannot change between its sessions without making
its rows incomparable.

**A chain may not finish while the gate is failing.** `Next: none` against a red gate is the
failure this feature is named for, so it stops the chain and says which — a different ending
from the four 0023 has, and a different exit code.

**What failed reaches the next session.** The gate's output is appended to the inherited
handoff under a heading the system prompt names, so a session starts knowing what it broke
rather than rediscovering it. Capped like any other tool result, from the same reserve.

**A repository with no gate is the normal case and is not an error.** With no command
configured the chain behaves exactly as it does today; the flag is what opts in.

## Tasks

- [ ] a chain runs a configured command after each session and records its exit code and
      output size on the session's row
- [ ] a gate that fails stops a chain that would otherwise have finished, with its own
      ending and exit code
- [ ] what the gate said reaches the next session, capped from the same reserve as a tool
      result
- [ ] a chain with no gate configured behaves as it does now, and a chain whose gate command
      cannot be run refuses at the start rather than after a session
- [ ] measured on the failure that motivated this: a chain against a repository whose gate
      catches the false claim does not report success

## Open questions

- Whether a red gate at the *start* of a chain should refuse to run at all. A repository
  that is already broken makes every session's verdict meaningless, but refusing would also
  stop the one instruction most worth giving — "fix the build".

## Log
- 2026-08-23 — dropped: verification of the deliverable is kit's side of the line. localcode owns runtime — window, ceiling, handoff, sandbox — and nothing about what a session delivered. The rule already exists in prose (run the gate, then kit audit, then kit ship) and went unenforced; the fix is that a task box names how it is checked and kit ship refuses on red, which closes it where it happens rather than one layer downstream and for autonomous sessions only.
