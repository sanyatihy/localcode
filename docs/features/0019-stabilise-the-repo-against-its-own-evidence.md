---
id: 0019
title: Stabilise the repo against its own evidence
status: Draft        # Draft | Accepted | Shipped | Dropped | Superseded
created: 2026-08-21
shipped:             # fill the date when status flips to Shipped
check:               # optional — date to check whether this worked. Only for bets.
checked:             # written by kit check <id> "<outcome>", never by hand
review: human
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
- **Nineteen sentences in `docs/TECH.md` run past 45 words**, which `kit audit` reports at
  LOW. Leaning: leave them. Each is a claim plus the one clause of evidence the protocol
  allows, and splitting them mechanically would cost the scoping the claims depend on.

## Log

- 2026-08-21 — the doc is written after the work, which inverts the protocol and is
  recorded rather than disguised. The request was a direct audit of an empty board, not a
  feature planned from the vision, so the branch is not a `kit claim` and does not carry
  this id.
- 2026-08-21 — **the MLX serving config was the configuration 0006 measured as unusable.**
  `mlx/config/mlx-4bit.env` shipped `PROMPT_CACHE_SIZE="16"`.
  `docs/data/2026-08-19-m2max-32gb-0006-mlx-16-slots.jsonl` has all 15 depth rows at
  `fail_over_budget`; `...-0006-runtime.jsonl`, the scored comparison at 2 slots, has all 15
  at `pass`. The raise landed in commit `003f7b7` at 08:27 UTC and the failing run is
  08:25–09:14 UTC, so the value was committed as the evidence against it was being taken and
  never corrected. Every MLX number in `docs/TECH.md` came from 2 slots. Set to 2.
- 2026-08-21 — **`cmd/report` printed one void run as three.** `accepted()` incremented a
  field on the aggregate, and `reportPaired` reads the baseline's set once per candidate.
  Reproduced: one stalled baseline run printed `(3 void: stalled past 4x the median)`. It is
  pure now and returns what it dropped. No published figure moves — the 0017 rows carry no
  stalls — but a void count is evidence here.
- 2026-08-21 — **a claim rested on rows that were not committed.** "Unbounded is not an
  option" was evidenced only by `results/0006-mlx-unbounded-cache.jsonl`, which
  `.gitignore` says is deleted with the worktree: 37 rows, free memory at 0.00, swap flat,
  9 runs over budget. Snapshotted. The section counts three misconfigurations and had
  evidence for two.
- 2026-08-21 — **`newestTranscript` could dereference a nil `FileInfo`.** It sorted glob
  results by `os.Stat` while discarding the error, so a transcript removed between the glob
  and the stat panics the driver. It scans for the newest and skips what it cannot stat.
- 2026-08-21 — **`cmd/handoff` wrote its rows through a second, unsynced appender**, where
  every other results writer syncs through `eval.AppendJSON`. A session is bounded at 30
  minutes, so a lost row is a lost half-hour.
- 2026-08-21 — **three documents said Claude Code cannot be offline.** 0010 falsified it: a
  fixture completed with the network denied in the kernel, and 0008 measured a real task
  contacting no host. A heading said it, `claude-code.env` deferred to "a later task" that
  had already run, and `docs/VISION.md` asserted it as a constraint. All three now state the
  measurement, scoped to token authentication.
- 2026-08-21 — **`docs/VISION.md` carried two figures its own features took away.** "Roughly
  a third of the machine unused" came from summed per-process RSS, which counts every shared
  page once per resident process; against anonymous-plus-wired the model wires 20.89 GB and
  an editor takes 6.61. Latency was named as the axis separating the profiles, before 0014
  found the GPU-wired one that sets the attended ceiling at 57,344.
- 2026-08-21 — **`Retrieval.Question` and `Patch.Package` were read by nothing**, while
  being declared, JSON-tagged and present in all fourteen fixtures. Removed from both.
  `TestHaystackUnchangedForPreDistractorFixtures` still passes, which is the proof no prompt
  moved.
- 2026-08-21 — **`make eval THINKING=off` could never have worked.** `cmd/eval` refuses the
  toggle without sampling and the Makefile had no variable to pass it. `SAMPLING` sits beside
  `THINKING` now, and `make help` renders the `##` comments already written for a target that
  did not exist.
- 2026-08-21 — **the checks that found nothing are recorded too.** `cmd/report` reproduces
  TECH.md's harness table cell for cell from the committed rows — 14/15, 12/15, 13/15, 15/15
  and ×0.21, ×0.55, ×4.02 — and the 0017 ratios 1.57×, 1.40×, 1.34×, 1.26× at acceptance
  3.94–3.97. No config label in `docs/data/` reports two served contexts, so the
  mislabelling guard held. Every link and anchor in all 33 markdown files resolves.
- 2026-08-21 — comments fell from 20.3% of the Go to 18.9%, 151 lines, by deleting
  restatement of `harness/README.md` and `docs/TECH.md` rather than by shortening anything.
  `internal/harness/hermes.go`, the case `docs/INBOX.md` named, went 35% to 22%. The INBOX
  question stands, and its evidence is updated: the ratio is the consequence, the
  duplication was the defect.
- 2026-08-21 — `docs/TECH.md`'s reorder is verified rather than reviewed: the multiset of
  non-blank lines differs by exactly the index and three cross-references. Two said "above"
  about a section that is below.
- 2026-08-21 — `kit audit` reported two MEDIUM findings and both are cleared: the work
  protocol was older than the installed kit's (`kit init`), and 0009 named 0004 in `needs:`,
  which is Dropped and will never ship. Three LOW `doc-bloat` findings are on Shipped docs
  and are left alone.
