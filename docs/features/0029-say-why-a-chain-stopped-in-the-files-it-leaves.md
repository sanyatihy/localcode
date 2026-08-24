---
id: 0029
title: Say why a chain stopped, in the files it leaves
status: Draft
created: 2026-08-24
shipped:
needs:
---

## Problem

A chain ends three ways — its handoff said `none`, two sessions planned the same step, or
the bound ran out — and only the supervisor's own narration says which. That narration goes
to a stream, so a chain run in the background leaves no record of how it ended. Reading a
completed 21-session chain back, the ending had to be reconstructed from `sessions.jsonl`
and the source of the guard that produced it.

## Non-goals

- No change to the exit codes. Bound, stall and timeout all mean "not finished" and one
  code for that is right; what is missing is which of them it was.
- No log file. The chain already writes a file per session and one row per session, and a
  third thing to find is worse than a field in the one a reader already opens.

## Design

The chain writes its ending where the run is already recorded: a `chain.json` beside
`sessions.jsonl`, holding the reason, the session it happened at, and the handoff a reader
should open. The reason is a fixed word — `finished`, `stalled`, `bound`, `timeout`,
`interrupted` — because the point is that a program can test it, and prose is what the
handoff is for.

Beside a file, not inside `sessions.jsonl`: a row describes a session, and the ending
belongs to the chain. `-continue` and `-resume` overwrite it, which is correct — a resumed
chain has a new ending and the old one is a session row away.

`localcode sessions` gains the reason in its line, which is the one place a reader looks
before deciding whether to resume.

## Tasks

- [x] a finished, stalled, bounded or timed-out chain records which it was, and where to read
- [x] `localcode sessions` shows a chain's ending beside its session count

## Log
