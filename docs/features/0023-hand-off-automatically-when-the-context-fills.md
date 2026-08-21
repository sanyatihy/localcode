---
id: 0023
title: Hand off automatically when the context fills
status: Draft        # Draft | Shipped | Dropped — kit ship and kit drop write it
created: 2026-08-21
shipped:             # written by kit ship, never by hand
needs:
---

## Problem

A `localcode` session that fills its context dies rather than handing off. 0016 refuses every
compaction, which frees no context, so Claude Code retries and is refused again until it hits
the hard limit and stops. Measured on one real session in a work repository: **20 refusals
over 17 minutes**, ending at `Prompt is too long` with the work abandoned. The refusal was
designed for the driven flow, where `cmd/handoff` starts the next session; interactive
`localcode` has no such driver, so the refusal has nothing to fall back on.

## Non-goals

- **Not a change to what a handoff contains.** 0016's three hooks and their format stand;
  this decides when a session ends and what starts the next one.
- **Not compaction.** Measured against this session: resuming from a handoff cost **4,265
  tokens and 54 s**, where compacting would have re-read 37,837 tokens — 452 s of ingest
  before generating a summary. Allowing compaction would trade a dead end for a recurring
  twelve-minute pause.
- **Not a bound on what fills the context.** 74.6% of that session was tool results and
  another 23.6% tool calls, so the fastest way to spend a window is unbounded command output.
  Capping it is a different decision and a `BACKLOG.md` line.
- **Not a driver for unattended work.** `cmd/handoff` already runs a task box across sessions
  on a budget. This is the interactive case, and it continues one instruction rather than
  working a list.

## Design

**`localcode` supervises the session it already launched.** The refusal cannot end a session
itself: exit 2 blocks the compaction and nothing else, and a hook returning
`{"continue": false}` was measured to change nothing — the session still died at
`Prompt is too long`. The launcher is the parent process, so ending and replacing the session
is its job rather than the hook's.

**The refusal log is the signal.** `pre-compact.sh` already appends to
`results/precompact.jsonl` under the state directory, and that file is the only trace a
refusal leaves. The supervisor watches it and needs no new channel.

**A refused compaction ends the session, and the next one starts from the handoff.** On the
first new refusal the supervisor terminates the session, waits for `SessionEnd` to write the
handoff, and launches a fresh one that `SessionStart` feeds it into. Measured: a terminated
session still runs `SessionEnd` and still writes its handoff.

**The developer is told, not asked.** One line naming what happened and which session this is
— seamless means the work continues, not that it happens invisibly.

**The instruction is re-issued; the handoff carries what was done.** A `-p` session gets its
original prompt again, since the handoff is what says how far it got. This is what
`cmd/handoff` already does with a task box, and it is why the handoff's `Next` field matters
more than its `Tried` field.

**Bounded, and the bound is a refusal rather than a silence.** `-max-sessions` caps the
chain, defaulting to a small number. A session that fills its context without advancing the
handoff would otherwise loop forever, and the honest failure is to stop and say which
handoff to read.

## Tasks

- [ ] `localcode` ends a session on the first refusal recorded during it, and reports why
- [ ] the next session starts from the handoff the terminated one wrote, with the original
      instruction re-issued
- [ ] the chain is bounded by `-max-sessions`, and reaching it stops with the handoff named
- [ ] `-no-handoff-chain` runs a single session, and a session that ends normally never
      starts another
- [ ] the README says what the developer sees when a session hands over

## Open questions

- Whether an interactive session should resume by replaying the developer's last message or
  by starting empty with the handoff. `-p` has one instruction and no ambiguity; an
  interactive session has a conversation, and only its last turn survives in the handoff.

## Log
