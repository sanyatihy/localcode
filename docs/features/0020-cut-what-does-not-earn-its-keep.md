---
id: 0020
title: Cut what does not earn its keep
status: Draft        # Draft | Accepted | Shipped | Dropped | Superseded
created: 2026-08-21
shipped:             # fill the date when status flips to Shipped
check:               # optional — date to check whether this worked. Only for bets.
checked:             # written by kit check <id> "<outcome>", never by hand
review:              # optional — `human` means a person merges this one. kit accept --review
needs:
related: 0012, 0014, 0019
---

## Problem

0019 held the repo's claims against its data. This holds its *structure* against the same
standard, and finds that three things the vision forbids are load-bearing.

This machine's limits live in three places. `config/machine.json` holds the headroom floor,
`internal/eval/tier2.go` hardcodes both desk ceilings, and nine `config/ladder-*.env` files
encode the rungs — the case the vision names first. `scripts/rungs.sh` derives those rungs
correctly, is documented as doing so, and is wired to nothing.

A config file claims an abstraction the code does not have. `config/profiles/` is documented
as owning the thinking mechanism, and two Go files hardcode `enable_thinking` instead. Five
of the profile's fields are read by nothing.

And a rule with one home has four. `stop_server` and `wait_healthy` were copied into four
scripts, where the second had already drifted into four different answers to the same trap.

## Non-goals

- **Not a re-measurement.** No server started and no model ran. Every figure is read off
  `docs/data/`, off git, or off a script run against the real machine.
- **No new capability.** Everything here already worked; the question was whether it earns
  the lines it costs.
- **The scratch in `results/` is still not touched.** 0019 left that decision with a human
  and it is still theirs.

## Design

**A number about this machine has one home, and it is `config/machine.json`.** The desk
ceilings move there beside the headroom floor; the ladder rungs stop being files at all, since
`scripts/rungs.sh` computes them from what the machine reports. `internal/eval` reads the file
and carries no constant of its own. The 128 GB machine then moves every ceiling by swapping
one file, which is what the vision asks for and what nine committed rungs prevented.

**A config file may only claim what the code reads.** `ThinkingSpec` keeps the two fields the
scorer now consults and loses the three it never did — including `effort_levels`, which was a
validation list against a documented decision *not* to validate. `LoadProfile` refuses a
mechanism it cannot perform, so a run cannot reach a request with the toggle silently unset
while its row claims a mode.

**A duplicate is deleted, not shortened.** Three configs were settings-identical under three
names and two more were a second copy of one rung. Generated cells replace committed ones; the
row already carries `ctx` and `kv`, so nothing about the record is lost.

**Two rejected runtimes are not top-level concerns.** `runtimes/` groups them the way
`harness/` groups the clients, and says in one line why a losing runtime is kept at all.

## Tasks

- [x] Every context ceiling and headroom floor this machine imposes is read from `config/machine.json`, and no Go file carries one
- [x] Ladder rungs are derived by `scripts/rungs.sh` and generated per cell, with the nine hand-written cells gone
- [x] `config/` holds only configs that differ from each other, and `make serve` starts the one the measurements settled on
- [x] The thinking toggle is rendered from the model profile, `enable_thinking` appears in no Go file, and a profile that cannot switch it is refused at load
- [x] `stop_server`, `wait_healthy`, `served_ctx` and the desktop baseline have one home in `scripts/lib.sh`
- [x] `docs/TECH.md` carries no sentence over 45 words, no session narration, and no superseded column
- [x] `make check` and `kit audit --strict` are clean

## Open questions

- **`scripts/ladder.sh` has never been run against generated cells**, only against committed
  ones. `cell_config` and `serve.sh` are verified together here, and `rungs.sh` derives the
  same 8k/16k/32k/64k it always did, but the full walk costs an hour of machine time. Leaning:
  let the next real ladder be the test, since the pieces are checked and a walk nobody needs is
  an hour spent proving a script.

## Log

- 2026-08-21 — written after the work, as 0019 was, and for the same reason: a direct audit of
  an empty board rather than a feature planned from the vision.
- 2026-08-21 — **the two desk ceilings move out of Go and into `config/machine.json`**, which
  `cmd/tier2` and the tests now read. They are numbers about one laptop; on a 128 GB machine a
  tier-2 sweep would have capped itself at this one's. `docs/TECH.md` records the new home.
- 2026-08-21 — **ladder rungs stop being files.** Nine were committed while `scripts/rungs.sh`
  derived them and was called by nothing, which is the defect the vision names first.
  `ladder.sh` generates each cell from `config/tuned.env`; a band stays askable as
  `CELLS="40960:q8_0 57344:q8_0"`.
- 2026-08-21 — **five of eighteen serving configs were the same configuration under other
  names.** `baseline.env`, `tuned.env` and `ladder-32k-q8_0.env` were byte-identical in every
  setting. `make serve` and `scripts/serve.sh` now default to `tuned.env`, which is a path
  other documents cite.
- 2026-08-21 — **a documented property of the scorer was one-third false.** The profile was
  said to own the thinking mechanism while `task.go` and `fidelity.go` hardcoded
  `enable_thinking` and `Profile.Thinking` was read by nothing. The toggle is rendered from the
  profile, and `LoadProfile` refuses a mechanism it cannot perform rather than leaving it
  unset. `effort_levels` went with it: a validation list against a decision not to validate.
- 2026-08-21 — **the assumption that four copies of a function are one function is falsified.**
  `wait_healthy` differed in all four scripts — in timeout, in how it detected death, and in
  how it read health — guarding a trap `docs/TECH.md` records as having already cost a wrong
  number. Both traps move to `scripts/lib.sh`.
- 2026-08-21 — the design gained a constraint the doing found: a generated cell must drop the
  base config's comments, which describe a context the cell does not serve. Caught by running
  `cell_config`, not by reading it.
- 2026-08-21 — the Go grows 44 lines net rather than shrinking, which the plan assumed it would
  not. `internal/eval/machine.go` is the cost of reading limits from config instead of
  declaring them, and it is the trade taken.
