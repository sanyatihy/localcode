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
- **No second memory instrument.** `internal/eval/mem.go` stays the one sampler, and gains
  wired, anonymous and total beside free and swap — the verdict needs a number that
  reflects pressure, and free memory is not one here. See the log.
- **No change to how a run is scored.** A row that swapped is still recorded exactly as it
  is today. The point is to stop the sweep starting, not to reinterpret its output.

## Design

The check is arithmetic on numbers this repo already has: total memory against what is
wired and anonymous, which is what competes for it. 0014 measured the model at 20.89 GB wired serving 32k and apps at 6.61 GB with
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

- [x] `internal/eval` gains a preflight that reads the machine's headroom — total less wired and anonymous — and reports whether a sweep can be carried, with the numbers it read
- [x] The headroom floor lives in a config file, and its default is justified by 0014's measured figures rather than chosen
- [ ] `cmd/eval` and `cmd/tier2` refuse to start when the preflight fails, and `-force` overrides it and marks the rows as forced
- [ ] A test covers both sides: a machine with headroom starts, one without is refused, and neither needs a running server

## Open questions

- Should a preflight failure be an exit code of its own, so a wrapper script can tell "the
  machine was not ready" from "the suite failed"? Leaning **yes**, reusing the existing
  convention that 2 means the run could not be carried out.

## Log
- 2026-08-20 — the floor is 4.0 GB, from 0014 rather than from taste: the model wired 20.89
  GB serving 32k and apps held 6.61 GB with an editor open, 27.50 against 32, so 4.5 GB is
  what a machine that was still usable had left. It is one configuration's figure, and the
  sampler only started recording headroom in the box above — a sweep's worth of readings is
  what should replace it.

- 2026-08-20 — rewrote the first two boxes to name headroom rather than free memory. The
  entry below corrected the design and left the boxes saying the opposite, which is the
  one place a builder actually reads.

- 2026-08-20 — the drafted non-goal was wrong, and a first build against it produced a
  check that could not pass — kept at `evidence/0015-first-run-no-handoff`: free memory is no
  pressure signal on macOS, as `mem.go` says in its own comment, and across 0010's 60-run
  sweep it read 0.16–0.65 GB whether the run swapped or not. A floor on it refuses every
  sweep this project runs. The verdict is now headroom — total less wired and anonymous,
  the arithmetic `docs/TECH.md` uses — which the sampler must read. What this changes
  downstream: the threshold task 2 puts in config is a headroom floor, and its default
  cannot be 0014's 11 GB figure, which is the budget for everything that is not the model
  rather than the margin a run needs.
