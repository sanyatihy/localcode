---
id: 0022
title: Regenerate every table in TECH.md from the rows behind it
status: Draft        # Draft | Shipped | Dropped — kit ship and kit drop write it
created: 2026-08-21
shipped:             # written by kit ship, never by hand
needs:               # feature ids that must ship first, e.g. 0002, 0003. Empty = can start now
---

## Problem

`docs/TECH.md` carries 16 tables and names not one of the files behind them, so no reader
can check a cell without guessing which of 21 `docs/data/` files to open. Two cells in the
most-cited table are wrong — the sampling sweep's `on/low` and `on/medium` wall figures say
18.6 and 22.4 minutes against rows summing to 14.6 and 14.4 — and they carry the "4× the
wall clock" conclusion that `README.md` repeats as settled. `docs/data/README.md` claims
`cmd/report` reproduces the harness comparison cell for cell; two of that table's five
columns are not columns the reporter prints.

## Non-goals

- **Not generating `docs/TECH.md`.** It is written, and its arguments are the point; the
  check compares numbers against a command's output and changes no prose.
- **Not re-measuring anything.** Every number here is already in `docs/data/`. A cell that
  disagrees with its rows is corrected to the rows, never re-run.
- **Not a provenance marker on tables no committed file backs.** The ladder and screen
  tables are assembled from apparatus rows and stay hand-derived; see the open question.
- **Not a new output format.** `cmd/report` gains columns, not a second mode — a reporter
  with a "table mode" that drifts from its normal one is the defect this feature exists to
  close.

## Design

**A table names its file and the command that produces it.** An HTML comment above each
backed table carries one shell line. That is the smallest thing that makes a cell checkable
by a human, and it is also what the gate reads, so the two cannot diverge.

**The reporter gains the two columns the harness comparison needs.** `context/turn`, and
wall averaged over clean rows with the count of them beside it. Both already exist in
`docs/TECH.md` and neither exists in `cmd/report`, which is what makes the regeneration
claim false today.

**The void rule becomes one threshold.** `cmd/report` voids a swapped run at `>20` MB in
its aggregate and at `>0` in the paired decode aggregate, and the comment on the first
argues against the second: macOS moves swap by a few MB without the run causing it. One
named constant, one predicate. It moves the Hermes clean-row count from 9 to 12 and its
wall from 307.1 s to 299.4 s, which is what `docs/TECH.md` already prints — so the table is
right and the code is two rules.

**"Unmeasured" names one thing.** The same output says "1 run(s) recorded no memory" and
"(13 unmeasured)" about the same 27 rows, meaning memory and decode respectively. The
second becomes "no decode measured".

**The gate asserts containment, not equality.** `scripts/tables.py` runs each marker's
command and fails when a numeric cell of the table below it is absent from the output. The
cost is that `docs/TECH.md` must quote the reporter's own formatting; the alternative —
parsing the reporter's output into a model the check re-renders — is a second renderer that
can drift from the first.

## Tasks

- [ ] The sampling table's two wall figures and the ratio they support match the rows in `2026-08-18-m2max-32gb-0005-toggle.jsonl`, in `docs/TECH.md` and in `README.md`
- [ ] A run is void by one threshold in one predicate, and the two senses of "unmeasured" have different words
- [ ] `cmd/report` prints `context/turn` and wall over clean rows with their count, so the harness comparison has a generator
- [ ] Every table in `docs/TECH.md` that a committed file backs carries the file and the command that regenerates it
- [ ] `make check` fails when a number in a marked table is not in the output of the command it names

## Open questions

- **Tables no command produces.** The ladder, desktop-band and screen tables read apparatus
  rows and per-cell fields that no reporter aggregates. Leaning: mark them
  `hand-derived from <file>` rather than write a generator per one-off table, and let the
  gate check only that the named file exists. Writing five single-use generators costs more
  than the drift it would catch.

## Log
