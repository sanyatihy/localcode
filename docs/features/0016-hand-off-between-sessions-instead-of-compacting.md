---
id: 0016
title: Hand off between sessions instead of compacting
status: Draft
created: 2026-08-19
shipped:
check:
checked:
review:
needs:
related: 0008, 0011
---

## Problem

A local model's usable window is smaller than a feature. Driving the settled harness against
the local endpoint on one task box, the session spent **17,417 tokens of tool output across
19 calls** working out where the code went — two command files read whole for one call site
each — and auto-compaction fired at **~26,000 tokens against a declared 45,056**, before a
box was ticked. Compaction then re-ingests the conversation to summarise it: at the measured
7.7 tok/s generation and ~86 tok/s prompt processing that is minutes of a session's life
spent reproducing what a commit already records.

## Non-goals

- **No change to `kit`.** Claims, boards and feature docs already carry state *between*
  features. What is missing is state between *sessions inside* one box.
- **No summarising.** The point is not a better compaction; it is not needing one.
- **No new agent and no orchestration layer.** One harness, run more than once.
- **No change to what a session may do.** This bounds how long a session lives, not what it
  is allowed to touch.

## Design

The repo is already the memory for anything that survives a feature: the doc's boxes and the
branch's commits. What has no home is **working state inside a box** — which files matter,
what was tried, what is next. That is what a compaction is trying to preserve, and a file
does it for nothing.

`HANDOFF.md` in the worktree holds it, and stays out of the branch: it is scratch, like
`results/`, and a reviewer should see commits rather than a diary.

Three hooks make it automatic rather than a thing the model must remember:

| hook | receives | what it does |
|---|---|---|
| `SessionStart` | `source` | prints `HANDOFF.md`, which is injected into the new session |
| `PreCompact` | `transcript_path`, `trigger` | refuses compaction (exit 2) and records that it fired |
| `SessionEnd` | `transcript_path` | writes a fallback handoff when the model did not |

**Refusing to compact is the load-bearing choice.** Compaction is the harness deciding to
spend generation on a summary; a bounded session that ends and hands off spends it on the
work instead. Refusing does not itself end a session — the driver's bound does — so what
the refusal buys is the generation, not the ending. The model is also told to keep
`HANDOFF.md` current, and the `SessionEnd` extraction is the net under that instruction
rather than the mechanism.

A driver then runs fresh sessions until the topmost box is ticked or a bound is reached.
Each starts near the preamble floor — 3,711 tokens for the four tools 0008 measured — rather
than wherever the last one ended.

**Whether this is worth it is measurable, and the same box is the measurement**: sessions
used, peak context per session, wall clock, and whether the box was finished at all.

## Tasks

- [x] `HANDOFF.md` is specified and ignored by git, and a `SessionStart` hook injects it into every new session
- [x] A `PreCompact` hook refuses compaction and records that it fired, so no session re-ingests itself
- [x] A `SessionEnd` hook writes a fallback handoff from the transcript when the model wrote none
- [x] A driver runs fresh sessions until the topmost box is ticked or a bound is hit, recording each session's peak context
- [ ] The mechanism is measured against the same box driven without it, and the outcome — including "not worth it" — is recorded in `docs/TECH.md`

## Log
- 2026-08-20 — a refused compaction leaves the session running: the turn completes and
  `PreCompact` fires again on the next one, once per turn while the conversation stays over
  the threshold. The leaning was that it dies, and the box said the refusal was what made a
  session end. It is not — the declared window is, by refusing a send it cannot fit — so the
  box now claims only what the refusal does, and bounding a session is the driver's box
  alone. The open question goes with the answer.
- 2026-08-20 — the `SessionStart` payload field is `source`, not `session_start_reason`;
  the table said the latter and no such field is sent. Nothing reads it yet — the hook
  fires on every source and branches on the file instead — but a matcher written against
  the wrong name would have matched nothing and failed silently.
