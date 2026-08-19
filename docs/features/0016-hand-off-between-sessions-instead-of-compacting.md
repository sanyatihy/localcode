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
| `SessionStart` | `session_start_reason` | prints `HANDOFF.md`, which is injected into the new session |
| `PreCompact` | `transcript_path`, `trigger` | refuses compaction (exit 2) and records that it fired |
| `SessionEnd` | `transcript_path` | writes a fallback handoff when the model did not |

**Refusing to compact is the load-bearing choice.** Compaction is the harness deciding to
spend generation on a summary; a bounded session that ends and hands off spends it on the
work instead. The model is also told to keep `HANDOFF.md` current, and the `SessionEnd`
extraction is the net under that instruction rather than the mechanism.

A driver then runs fresh sessions until the topmost box is ticked or a bound is reached.
Each starts near the preamble floor — 3,711 tokens for the four tools 0008 measured — rather
than wherever the last one ended.

**Whether this is worth it is measurable, and the same box is the measurement**: sessions
used, peak context per session, wall clock, and whether the box was finished at all.

## Tasks

- [ ] `HANDOFF.md` is specified and ignored by git, and a `SessionStart` hook injects it into every new session
- [ ] A `PreCompact` hook refuses compaction and records that it fired, so a session ends rather than re-ingesting itself
- [ ] A `SessionEnd` hook writes a fallback handoff from the transcript when the model wrote none
- [ ] A driver runs fresh sessions until the topmost box is ticked or a bound is hit, recording each session's peak context
- [ ] The mechanism is measured against the same box driven without it, and the outcome — including "not worth it" — is recorded in `docs/TECH.md`

## Open questions

- Does a refused compaction leave the session able to continue, or does it die on the next
  turn? Leaning **it dies**, which is acceptable because the handoff is already written — but
  it is the first thing the first box should establish, since the whole design rests on it.

## Log
