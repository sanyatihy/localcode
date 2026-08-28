---
id: 0045
title: Leave a visited repository exactly as the session found it
status: Draft
created: 2026-08-27
shipped:
needs:
---

## Problem

`cmd/localcode`'s own package doc promises that a visited repository "ends a session with
exactly the files the work changed". The movement probe breaks it: `git status --porcelain`
refreshes the index stat cache, so `.git/index` is rewritten twice a session, per worktree,
by the instrument that exists to observe whether the session changed anything.

## Non-goals

- **Not dropping the probe.** What it measures — whether a session moved the repository —
  is what stops a chain that has stopped converging, and 0035 measured commits stopping
  five sessions before movement did.
- **Not a faster probe by measuring less.** Reading every worktree was a measurement: a
  chain working through `kit` edits in a linked one, and 44 calls across two files read as
  having done nothing until the probe looked there.

## Design

**`--no-optional-locks` on every read.** Measured on a scratch repository with a dirty
working tree: `git status --porcelain` moved `.git/index`'s mtime, and
`git --no-optional-locks status --porcelain` did not. The flag tells git not to take the
index lock, which is what the refresh needs, so the observation stays identical and the
write stops. It is the whole fix for the promise.

**It is also the fix for a contention the probe can lose.** The lock the refresh takes is
the one the session's own `git` wants, and the probe runs while nothing else should be
touching the repository — but a session that ended mid-command, or a developer with the
same checkout open, is exactly when a chain must not stall. 0043 bounded that call at ten
seconds; not taking the lock is better than timing out on it.

**Every git the probe runs, not the status alone.** `count-objects`, `rev-list` and
`worktree list` are reads too, and a flag applied to one of four is a rule nobody can state.
It goes on the shared `git` helper, which is the one place all four pass through.

## Tasks

- [ ] The probe leaves `.git/index` untouched, asserted against a repository with a dirty
      working tree, and still reports movement and commits as it did

## Open questions

- **Whether the probe should read every worktree every time.** On this repository that is
  eight git processes per reading and sixteen per session; the cost is invisible here and
  is not on a large monorepo. Leaning: leave it. The number that would justify caching is
  the wall clock of a reading against a session's minutes, and nobody has taken it.

## Log
