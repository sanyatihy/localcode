---
id: NNNN
title: <short imperative name>
status: Draft        # Draft | Accepted | Shipped | Dropped | Superseded
created: YYYY-MM-DD
shipped:             # fill the date when status flips to Shipped
check:               # optional — date to check whether this worked. Only for bets.
checked:             # written by kit check <id> "<outcome>", never by hand
review:              # optional — `human` means a person merges this one. kit accept --review
needs:               # feature ids that must ship first, e.g. 0002, 0003. Empty = can start now
related:             # other docs worth reading first
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
     Any of those means the second feature needs the first, however separate they look.

     `related:` is different and does not affect ordering: it is reading material. -->


## Problem
<!-- 1-3 sentences: the standing tension and the signal that justifies it now — not the
     incident that surfaced it, which the pull request holds. Shorter and nobody can tell
     whether it was worth doing; longer and the doc is arguing instead of deciding. -->

## Non-goals
<!-- What this deliberately does not do, and where that lives instead. One line each;
     a reason with each, or the next person reopens the argument. -->

## Design
<!-- Decisions only: what was chosen, what it beat, what it costs. Not a walkthrough — the
     code will say what the code does, and this is read alongside it. Call out data-model,
     migration, API and config changes explicitly. Written before the work, so it is the
     part most likely to be wrong by the end; let `## Log` carry what the doing teaches.

     One claim per sentence, and evidence gets one clause. Rejected, then the rewrite:
       no   the cache is per-process because a shared one would need invalidation across
            workers, which this deployment does not have, and it also drops the dependency
            we have been trying to get rid of since the last migration
       yes  The cache is per-process. A shared one needs cross-worker invalidation, which
            this deployment does not have.
     The third clause was about the work rather than the system, so it is gone rather
     than split. -->

## Tasks
<!-- Each box is one PR-sized outcome you can verify without asking anyone, in the
     order they should be done:
       - [ ] the outcome, not the activity

     Guidance lives in comments here on purpose. A box written inside one is invisible
     to the tool, so an unwritten plan counts as unwritten: `no-tasks` says the plan is
     missing instead of `kit next` handing an agent a placeholder to build. -->

## Open questions
<!-- Optional. Delete when empty. Each with a leaning if you have one. -->

## Log
<!-- One entry per change to the plan, and each names the change: a box rewritten, a
     premise falsified, an open question settled, a number something downstream reads.
     Take the room a change needs; append-only.

     An entry naming no change is not an entry. What the work did before it converged —
     the run, the re-run, the reading that was revised — belongs in the pull request, and
     a fact still true after this ships belongs in the as-built doc, one line. Never both:
     each fact has one home.
       - YYYY-MM-DD — decided X over Y because … -->

