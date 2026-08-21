---
id: 0023
title: Record the runs the instruments could not take
status: Draft        # Draft | Shipped | Dropped — kit ship and kit drop write it
created: 2026-08-21
shipped:             # written by kit ship, never by hand
needs:               # feature ids that must ship first, e.g. 0002, 0003. Empty = can start now
---

## Problem

`internal/eval` builds a `fail_server_error` row carrying the wall clock a failed request
spent, commented "recording zero would understate any total this row is summed into", and
`cmd/eval` discards it and writes nothing. A sweep against a dead endpoint therefore
produces no results file at all and exits **1**, the code its own contract reserves for
tasks that failed; `Makefile` and `scripts/pair.sh` both end that call in `|| true`, so the
sweep proceeds to `cmd/report`, which summarises the rows the file already held and exits 0.
The same silence sits in the committed record: 399 of 834 rows predate the memory
instrument and nothing outside `cmd/report`'s own output says so.

## Non-goals

- **Not re-running the 399 uninstrumented rows.** They are labelled, not replaced — the
  same treatment the page-size erratum already has in `docs/data/README.md`.
- **Not retrying a failed request.** A dropped connection stays one lost observation; what
  changes is that the observation is lost *in the file* rather than nowhere.
- **Not aborting a suite on the first failure.** Losing hours to one dropped connection is
  still worse than a gap, and that reasoning stands.
- **Not a new outcome.** `fail_server_error` already exists and `cmd/tier2` already records
  it; this makes tier 1 agree with tier 2.

## Design

**A gap is a row.** The `Result` the client already returns for a transport failure is
appended like any other. Nothing else in the pipeline changes: `cmd/report` already counts
`fail_server_error` in its failure breakdown, so a sweep that half-ran reads as a sweep that
half-ran.

**Exit 2 becomes reachable.** `cmd/eval` counts a transport failure apart from a scored
one and returns a distinct sentinel, so "the measurement could not be taken" and "the answer
is no" stop sharing a code. That is the whole reason the contract exists, and no caller can
use it while both map to 1.

**The callers branch instead of swallowing.** `make eval` and `scripts/pair.sh` stop on 2
and continue on 1. `|| true` was there so a failing suite still gets summarised, which is
right for 1 and wrong for 2: a pair whose second side never served produces a ratio computed
from one side.

**The record carries its own scope.** The six files with no `swap_delta_mb` or
`mem_measured` field get a line in `docs/data/README.md`, and the four `docs/TECH.md`
sections that cite them — sampling and thinking, reasoning effort, which tier-1 tasks carry
signal, tool-call adherence — say the contamination rule could not be applied. This is a
scope note, not a retraction: no conclusion in those sections changes.

## Tasks

- [ ] A transport failure appends a `fail_server_error` row carrying the wall clock it spent
- [ ] `cmd/eval` exits 2 when the run could not be carried out, and a test covers the case its package comment names first
- [ ] `make eval` and `scripts/pair.sh` stop on exit 2 rather than reporting the rows a previous run left
- [ ] The six files with no memory instrumentation say so in `docs/data/README.md`, and every `docs/TECH.md` section resting on them carries the scope
- [ ] `cmd/report` documents its exit codes, which `README.md` already claims every command does

## Open questions

None.

## Log
