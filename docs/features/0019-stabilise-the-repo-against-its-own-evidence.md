---
id: 0019
title: Stabilise the repo against its own evidence
status: Draft        # Draft | Accepted | Shipped | Dropped | Superseded
created: 2026-08-21
shipped:             # fill the date when status flips to Shipped
check:               # optional — date to check whether this worked. Only for bets.
checked:             # written by kit check <id> "<outcome>", never by hand
review:              # optional — `human` means a person merges this one. kit accept --review
needs:
related: 0006, 0010, 0014, 0017
---

## Problem

Eighteen features have shipped or been dropped and nothing is in flight, so every claim
here is settled and can be held against the data that settled it. That finds four defects a
reader cannot. A serving config is the configuration a run measured as unusable. A reporter
inflates the count of runs it voids. A claim's rows are not committed. And three documents
assert the opposite of a later measurement. An empty board is also what makes the audit
safe: it changes nothing anybody is building on.

The other half is legibility. `README.md` was one line, `docs/TECH.md` had grown to 21
sections with no order and no index, and five directories had no entry point. The local
tier reads this repository on a bounded window, so prose restating prose is context spent
twice.

## Non-goals

- **Not a re-measurement.** No server was started and no model ran. Every number here is
  read off `docs/data/`, and where a table could be regenerated it was — see the Log.
- **Not a restructure.** 0013 and 0014 name `config/ladder-*.env` by path and are frozen, so
  moving files would falsify shipped history to tidy a listing.
- **No edits to Shipped docs.** Three `doc-bloat` findings from `kit audit` sit on frozen
  docs and are left, correctly, unfixed.
- **Not a comment budget.** The ratio fell as a consequence of deleting restatement, not as
  a target. `docs/INBOX.md` carries why a ratio is the wrong instrument.
- **The scratch in `results/` is not touched.** It is untracked and unrecoverable, so
  deleting it is the human's call, not an agent's.

## Design

**A claim is checked against the rows, not against the prose beside it.** The method is the
one the repo already applies to models: regenerate the table with the committed tool and the
committed data, and treat a disagreement as a defect in whichever of the two is wrong. That
is what found the MLX config — `docs/TECH.md` and `docs/data/README.md` both already said
16 slots does not fit here, and only `mlx/config/mlx-4bit.env` disagreed.

**Where a fact has two homes, the one that is not the source of truth goes.** Comments that
restated `harness/README.md` or `docs/TECH.md` are deleted rather than shortened; superseded
tables in harness READMEs are replaced by a pointer to the ranked result rather than kept
beside it. The rule is already in `AGENTS.md` for docs; this applies it to code and to
READMEs.

**Restructuring `docs/TECH.md` moves headings, never words.** The reorder is mechanical —
sections are parsed, regrouped and re-emitted — and verified by comparing the multiset of
non-blank lines before and after. Three lines differ, and each is a directional
cross-reference that pointed the wrong way.

**New prose only where a directory has no entry point.** No per-feature index: title and
status live in the frontmatter, and a second copy would drift.

## Tasks

- [x] The paired decode reporter counts a stalled run once however many times it is read, with a regression test that reads the baseline twice
- [x] `cmd/handoff` survives a transcript that vanishes mid-glob, and its rows reach disk with the same durability as every other results file
- [x] Fields no code reads are gone from the Go and from all fourteen fixtures, with the pinned haystack hashes proving no prompt moved
- [x] `go test` and the build-failure markers have one home each, and the chat and Messages paths share one reply type
- [x] `make eval THINKING=off SAMPLING=nonthinking` runs, and `make help` lists the targets
- [x] `mlx/config/mlx-4bit.env` serves the slot count the scored comparison ran on, and the rows behind "unbounded is not an option" are in `docs/data/`
- [x] Every document that claimed Claude Code cannot be offline states what was measured instead, scoped to token authentication
- [x] `docs/VISION.md` no longer carries the two figures later features falsified
- [x] `docs/TECH.md` has an index and an order, and no cross-reference points the wrong way
- [x] `README.md` says what the project is, what it settled, and how to start it; `config/`, `scripts/`, `tasks/`, `mlx/` and `mtplx/` each have an entry point
- [x] `kit audit --strict` is clean and `make check` is green

## Open questions

- **`results/` holds 11 untracked scratch files, 288 KB.** Nine are contained row-for-row in
  a `docs/data/` snapshot. The other two are intermediates nothing cites: a 5-row MLX depth
  smoke, and 2 truncated rows from a 0013 re-run. Leaning: delete, once a human agrees.

## Log

- 2026-08-21 — written after the work, which inverts the protocol: a direct audit of an empty
  board rather than a feature planned from the vision. The branch is not a `kit claim` and
  does not carry this id.
- 2026-08-21 — **the premise that a committed config records what was measured is falsified.**
  `mlx/config/mlx-4bit.env` served `PROMPT_CACHE_SIZE="16"`, which 0006 measured as unusable:
  15/15 depth rows `fail_over_budget` against 15/15 `pass` at 2 slots, the count every MLX
  number in `docs/TECH.md` came from. Set to 2. The raise landed at 08:27 UTC while the run
  refuting it was taken 08:25–09:14, so the config was never corrected when it failed.
- 2026-08-21 — **a claim rested on rows the repo does not keep.** "Unbounded is not an option"
  was evidenced only by a file in gitignored `results/`. Snapshotted as
  `docs/data/2026-08-19-m2max-32gb-0006-mlx-unbounded-cache.jsonl`, so the MLX section's three
  misconfigurations are three rather than two.
- 2026-08-21 — **the vision asserted the opposite of a measurement.** `docs/VISION.md` and two
  harness documents said Claude Code cannot be offline; 0010 completed a fixture with the
  network denied in the kernel, and 0008 measured a real task contacting no host. The
  constraint is rewritten to what was measured, scoped to token authentication.
- 2026-08-21 — **two of the vision's figures were taken away by its own features.** "Roughly a
  third of the machine unused" came from summed per-process RSS, which counts a shared page
  once per process; latency was named as the only axis separating the profiles before 0014
  found the GPU-wired one that sets the attended ceiling at 57,344. Both bullets now state
  what was measured.
- 2026-08-21 — the void count `cmd/report` prints now counts a stalled run once however many
  times its side is read. `accepted()` mutated the aggregate and the baseline is read once per
  candidate, so one stall printed as three. No published figure moves: the 0017 rows carry
  no stalls.
- 2026-08-21 — the `results/` question is narrowed: nine of its eleven files are contained
  row-for-row in a `docs/data/` snapshot, and the two that are not are intermediates nothing
  cites. The leaning to delete is unchanged.
- 2026-08-21 — the open question about long sentences is settled the other way, in 0020. The
  claim that splitting them would cost the scoping did not survive doing it: all nineteen
  split without losing a clause.
