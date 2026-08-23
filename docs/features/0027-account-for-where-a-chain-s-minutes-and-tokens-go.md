---
id: 0027
title: Account for where a chain's minutes and tokens go
status: Draft        # Draft | Shipped | Dropped — kit ship and kit drop write it
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

Deciding anything about a chain currently means reconstructing it by hand. The decode rate
behind a two-hour run was recovered by differencing transcript timestamps, the preamble tax
by reading one turn's `input_tokens`, and the split between ingest and generation by
arithmetic on both — none of which the tool reports, and all of which the next optimisation
has to be judged against.

## Non-goals

- **Not a second scorer.** `cmd/eval` and `cmd/report` score the model against a task suite;
  this reports what one chain of real work cost, which is a different question and reads a
  different file.
- **Not new instrumentation in the session.** Everything needed is already written down:
  `sessions.jsonl`, the per-session `session.json`, and the transcript the gate already
  reads for its ceiling.
- **Not a judgement.** It reports the split; whether a number is good is what the features
  that change it argue about.

## Design

**One command over a chain's own files.** `sessions.jsonl` has wall clock, peak context,
turns and calls per session; the transcript has per-turn `input_tokens` and
`output_tokens`. Between them the four numbers that decide things: tokens generated, tokens
ingested, the preamble paid once per session, and wall clock attributable to decode.

**Decode rate is derived from the transcript's own timestamps, client-side.** That is the
harness's existing rule for speculative decoding and it applies for the same reason: a
server-reported rate does not include what the harness spends around the call.

**The preamble tax is the number worth naming.** It is the first turn's input, paid again by
every session in the chain, and it is what a smaller ceiling multiplies — so it is the term
that decides whether a shorter context can pay for itself.

**JSONL out, as every other measurement here.** A chain worth reporting is a chain worth
committing to `docs/data/`, and the shape has to be the one `docs/data/README.md` already
describes.

## Tasks

- [x] a command reports one chain: sessions, wall clock, tokens generated, tokens ingested,
      preamble paid per session, and the decode rate its timestamps imply
- [x] the same numbers per session, so a chain that went wrong shows where
- [ ] it reads a chain that is still running without waiting for it to finish
- [ ] the output is the JSONL shape `docs/data/` takes, and one real chain is committed
      there

## Open questions

- Whether wall clock outside the model — tool execution, the harness's own work — is worth
  separating from ingest. It is the residual after decode and ingest are accounted for, and
  on the one chain measured so far it was small enough to be noise or a real 10%.

## Log
