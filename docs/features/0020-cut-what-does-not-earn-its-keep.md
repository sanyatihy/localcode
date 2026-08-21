---
id: 0020
title: Cut what does not earn its keep
status: Draft        # Draft | Accepted | Shipped | Dropped | Superseded
created: 2026-08-21
shipped:             # fill the date when status flips to Shipped
check:               # optional — date to check whether this worked. Only for bets.
checked:             # written by kit check <id> "<outcome>", never by hand
review: human
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
- 2026-08-21 — **`internal/eval/tier2.go` hardcoded 57,344 and 65,536**, the two numbers
  `config/machine.json` exists to hold. On a 128 GB machine a tier-2 sweep would have capped
  itself at this laptop's ceilings. Both now come from the file, and the tests read the
  committed one — so `TestCommittedMachineDeclaresBothDeskProfiles` asserts a property of the
  config rather than of a constant restated in the test.
- 2026-08-21 — **nine ladder rungs were committed as files while `scripts/rungs.sh` derived
  them and was called by nothing.** The vision names a hardcoded rung as the first defect to
  fix. `ladder.sh` now takes its cells from `rungs.sh` and generates each from
  `config/tuned.env`; a band is still askable as `CELLS="40960:q8_0 57344:q8_0"`. Verified:
  `rungs.sh` derives the same rungs, and a generated cell drives `serve.sh` to the same flags.
- 2026-08-21 — **five of the eighteen serving configs were duplicates.** `baseline.env`,
  `tuned.env` and `ladder-32k-q8_0.env` were byte-identical in every setting, as were
  `ctx16k-f16.env` and `ladder-16k-f16.env`. A second name for one configuration can only ever
  disagree with the first. `make serve` now starts `tuned.env`, which is what the project
  concluded rather than where it began.
- 2026-08-21 — **the scorer's model-portability claim was one-third false.** `docs/TECH.md`
  said the thinking mechanism lives in the profile; `task.go` and `fidelity.go` hardcoded
  `enable_thinking`, and `Profile.Thinking` was read by nothing. The toggle is rendered from
  the profile now, and `LoadProfile` refuses a mechanism it cannot perform rather than leaving
  it unset. `effort_levels` and `effort_default` are gone: a level is passed to the server
  verbatim by an explicit decision, so a list in config was a gate this project had chosen not
  to have. `Sampling.MinP` was declared and mentioned nowhere else at all.
- 2026-08-21 — **four copies of two functions had drifted into four answers.** `stop_server`
  was identical in three scripts and different in the fourth; `wait_healthy` differed in all
  four — in timeout, in how it detected death, and in how it read health. Both traps are in
  `docs/TECH.md` and neither may be re-solved locally, so both now live in `scripts/lib.sh`.
  `screen.sh` keeps a pid-based `server_alive` override, because a runtime that sets its own
  process title cannot be found by pattern.
- 2026-08-21 — **generated cells first inherited the base config's comments**, which described
  a context the cell did not serve. Caught by running `cell_config` rather than reading it. A
  generated file explaining itself as something else is worse than one that says nothing, so it
  writes its own one-line header and copies settings only.
- 2026-08-21 — `docs/TECH.md` loses 48 lines and gains an honest shape. Four sections on the
  two-tier split become one: the run-by-run table of three sessions was narration the protocol
  puts in the pull request, and it carried a comment-ratio figure 0019 had already falsified.
  The reasoning-effort section printed an `off` column and then spent ten lines explaining that
  it is void — the column is gone — and one paragraph stated the same finding twice, verbatim.
  Nineteen sentences over 45 words are split; `kit audit` reports none anywhere now.
- 2026-08-21 — `scripts/matrix-request-level.sh` produced the labels of the 2026-08-17 matrix
  that 0005 superseded, and hand-wrote the sampling pair that `-sampling-profile` exists to
  own. Deleted: keeping it kept alive the mistake that voided 114 rows.
  `harness/claude-code/editor-session.md` was a session transcript whose every number is in
  `docs/TECH.md`; its one unique figure, 45 s in a terminal against 462 in the extension, moved
  there and the diary went.
- 2026-08-21 — the Go grew 44 lines net. `internal/eval/machine.go` is new and the deletions
  do not cover it, which is the honest trade: a config-driven machine and a profile-driven
  toggle cost more code than the constants they replace.
