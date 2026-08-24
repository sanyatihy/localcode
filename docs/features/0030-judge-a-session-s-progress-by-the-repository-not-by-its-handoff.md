---
id: 0030
title: Judge a session's progress by the repository, not by its handoff
status: Draft
created: 2026-08-24
shipped:
needs: 0029
---

## Problem

The chain's only stall test is that two sessions in a row wrote the same `**Next:**`. It
compares prose a model wrote, and a model rewords: measured on a 25-session chain, eight
consecutive sessions produced no commit at all while the guard stayed silent, because each
restated the same instruction differently. It fired at the eighth, once two handoffs finally
matched byte for byte, having spent about 49 minutes on sessions that changed nothing.

## Non-goals

- No removal of the `Next` comparison. It is cheap and it caught the identical pair; this
  adds a second test rather than replacing the first.
- No judgement of whether the work was *good*. Whether a commit was worth making is what a
  review is for, and a supervisor that guessed at it would stop chains doing real work.

## Design

A session made progress when the repository it was given moved: a new commit on any branch,
or a change to the working tree that was not there when the session started. `HEAD` alone is
wrong — a chain that works in a claimed worktree leaves `HEAD` still. Counting objects
rather than files is what makes a commit in a worktree count, and a session that only read
count as nothing.

Two consecutive sessions with no movement stop the chain, matching the existing guard's
patience. Two, not one: a session that spends its budget reading before it edits is normal,
and the failure being caught is a chain that has stopped converging, not a slow session.

The backlog entry this promotes names the case that makes a naive test wrong: a chain whose
work is a question rather than an edit — a measurement, an investigation — never commits and
would be stopped on its second session. So the test is *repository moved OR the handoff's
`Next` changed*, and only a chain failing both is stalled. That keeps prose as the fallback
for work that leaves no trace, and makes the repository authoritative for work that does.

## Tasks

- [x] a session reports whether the repository moved while it ran
- [x] two sessions in a row that leave the repository as they found it stop the chain, once
      that chain has moved it at all

## Log
- 2026-08-24 — the second box is rewritten, because the design's own rule contradicts the
  problem it names. `repository moved OR the handoff's Next changed` scores a reworded step
  as progress, and rewording is exactly what the eight silent sessions did — the rule as
  written would have let that chain run. What ships is the design's other sentence, `two
  consecutive sessions with no movement stop the chain`, with the prose fallback applied
  where it is needed: a chain that has never moved a repository is judged on its handoffs,
  and one that has is judged on the repository. A chain doing question-shaped work is
  protected by never having moved anything, not by rewording its plan.
