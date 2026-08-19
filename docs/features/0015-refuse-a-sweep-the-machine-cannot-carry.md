---
id: 0015
title: Refuse a sweep the machine cannot carry
status: Draft
created: 2026-08-19
shipped:
check:
checked:
review:
needs:
related: 0010, 0014
---

## Problem

0010's sweep spent two hours producing timings that measured the pager: at 65,536 with an
editor resident the machine swaps, and the vision calls a swapped run void rather than
slow. The rows recorded it faithfully — 25 of 60 grew swap — but nothing said so until the
sweep was over, and by then the machine time was spent. The scorer knows what the machine
has before it starts, and says nothing.

## Non-goals

- **No scheduling and no waiting.** This refuses or warns; it does not sleep until memory
  frees up, and it does not decide when a better moment is.
- **No new memory instrument.** `internal/eval/mem.go` already samples free memory and
  swap; this is a decision made from what it already reports.
- **No change to how a run is scored.** A row that swapped is still recorded exactly as it
  is today. The point is to stop the sweep starting, not to reinterpret its output.

## Design

The check is arithmetic on numbers this repo already has: swap in use before the run, and
free memory. 0014 measured the model at 20.89 GB wired serving 32k and apps at 6.61 GB with
an editor open, against 32 GB — so roughly 11 GB is left for everything that is not the
model, and a browser does not fit in it.

**A preflight that refuses is worth more than a warning nobody reads**, because the cost it
prevents is hours rather than seconds. It runs once, before the first task, and it fails
loudly with the numbers it read. `-force` runs anyway and records that it was forced, since
a measurement someone deliberately wants is not the scorer's to veto.

**The threshold is a config value, not a constant in the code.** What counts as too little
headroom is a property of the machine, and this repo already keeps machine properties in
`config/`. A threshold in Go would be re-derived by anyone on other hardware.

**The check is the same for tier 1 and tier 2**, which means it belongs in
`internal/eval` beside the sampler rather than in either command.

## Tasks

- [ ] `internal/eval` gains a preflight that reads free memory and swap and reports whether a sweep can be carried, with the numbers it read
- [ ] The threshold lives in a config file, and its default is justified by 0014's measured figures rather than chosen
- [ ] `cmd/eval` and `cmd/tier2` refuse to start when the preflight fails, and `-force` overrides it and marks the rows as forced
- [ ] A test covers both sides: a machine with headroom starts, one without is refused, and neither needs a running server

## Open questions

- Should a preflight failure be an exit code of its own, so a wrapper script can tell "the
  machine was not ready" from "the suite failed"? Leaning **yes**, reusing the existing
  convention that 2 means the run could not be carried out.

## Log
