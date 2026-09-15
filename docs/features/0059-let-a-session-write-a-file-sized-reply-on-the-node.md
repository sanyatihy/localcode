---
id: 0059
title: Let a session write a file-sized reply on the node
status: Submitted
created: 2026-09-15
submitted: 2026-09-15
needs:
---

## Problem

On the node a session writing a whole module in one `Write` was cut at the 4,096-token
output cap, Claude Code rejected the truncated call and retried it three times at three
and a half minutes each, and the session produced nothing more. The cap was sized for
the laptop's decode rate; the node decodes twice as fast, four times with the head.

## Non-goals

- A cap that follows the endpoint. One committed file serves both machines; the laptop
  pays the same fifteen minutes for a truncated-and-retried reply as for a complete one.
- Ending the launcher's process group on a kill. Recorded in BACKLOG, not changed here.

## Design

`CLAUDE_CODE_MAX_OUTPUT_TOKENS` is 8,192. The prompt budget is the window less this, so
at 49,152 it is 40,960 instead of 45,056, and the launcher's arithmetic follows it
without change. TECH's budget paragraph records the new reservation and why.

## Tasks

- [x] Let a session write a file-sized reply on the node

## Log

<!-- What the doing taught that a plan would not have predicted: a premise falsified, an
     approach abandoned, a measurement that changed the shape. -->
