---
id: 0035
title: Judge a session by every worktree, and give a chain a third try
status: Shipped
created: 2026-08-24
shipped: 2026-08-24
needs:
---

## Problem

0030's movement test counts the object database, which every worktree shares, but reads the
working tree of one — the directory the chain was started in. A project worked through `kit`
does its editing in a linked worktree, so a session that writes code and ends before
committing reads as having done nothing. Measured on a live chain: a session spent 44 tool
calls over 29 minutes writing 140 lines across two files, and its row records
`repo_moved: false`.

## Non-goals

- No change to the object count. It already catches a commit made in any worktree, which is
  the half of the test that works.
- No inspection of what a working tree holds. Whether the files are worth anything is a
  review's question; this asks only whether they are there.

## Design

The working-tree half reads every worktree the repository has, not the one the chain was
started in. `git worktree list --porcelain` names them and each is asked for its own status,
so the same commit that already registers is joined by the edits that precede it.

A worktree git lists but cannot stat is skipped rather than failing the reading. A stale
entry is a repository's own housekeeping, and a probe that refuses to answer because of one
would stop a chain for a reason that has nothing to do with the session.

Three consecutive still sessions stop the chain rather than two. Two was matched to the
`Next` comparison's patience, before there was evidence about how a real session behaves:
on a chain of ten, single still sessions were common and none of the pairs was a stall. The
cost of a third is one session; the cost of stopping a chain that was working is the chain.

## Tasks

- [x] a session that edits a linked worktree and commits nothing reports the repository moved
- [x] a chain stops after three still sessions rather than two

## Log
