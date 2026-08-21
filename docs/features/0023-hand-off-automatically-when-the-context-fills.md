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
  twelve-minute pause. Pi, which compacts by design, is not obviously better off here: its
  defaults reserve 16,384 tokens and keep 20,000 recent ones, so against the 32,768 its
  provider block declares it would trigger at 16,384 and try to keep more than it had.
  That reading is from the source and its defaults, not from a run, and it stays a
  hypothesis until one — but it is enough that compaction is not the obviously-correct
  answer somebody else already found.
- **Not a bound on what fills the context.** 74.6% of that session was tool results and
  another 23.6% tool calls, so the fastest way to spend a window is unbounded command output.
  Pi caps a tool result at 50 KB or 2,000 lines and spills the rest to a file the model may
  read, which treats the cause where this feature treats the consequence. It is the better
  fix and it is a `BACKLOG.md` line rather than a box here, because it is one hook and this
  feature is already large.
- **Not a driver for unattended work.** `cmd/handoff` already runs a task box across sessions
  on a budget. This is the interactive case, and it continues one instruction rather than
  working a list.
- **Not `claude --resume`.** Resuming re-ingests the conversation that just failed to fit,
  which is the cost being avoided. A session starts clean and inherits a handoff.

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
more than its `Tried` field. Re-issuing is also what stops the goal drifting: the instruction
is quoted from the developer every time rather than paraphrased through a chain of handoffs.

**A session never inherits and writes the same file.** The supervisor archives the handoff
into `handoffs/NNNN.md` before launching, so `HANDOFF.md` is absent at session start and
`session-end.sh` always writes one. `SessionStart` reads the newest archive when there is
one. That is what makes the second handoff in a chain describe the second session.

**Long chains carry a digest, not their whole history.** `SessionStart` injects the newest
handoff whole and one line from each of the previous four — enough to see what has already
been ruled out, bounded so the injection cannot grow with the chain. The newest handoff
measured 452 tokens against a 45,056-token window, so the budget is not the constraint; what
is read again at every session start is.

**A chain that stops advancing is stopped rather than bounded.** Two consecutive handoffs
with the same `Next` mean the sessions are repeating each other, which `-max-sessions` would
hide behind a count. The run ends and names the handoff to read.

**A handoff belongs to a chain, and a repository carries as many chains as it has lines of
work.** One handoff per repository assumes one thing is being worked on there, and two
unrelated tasks in the same checkout would overwrite each other's state. A chain is one
`localcode` invocation and every session its handovers produce.

**Starting clean is the default, because that is Claude Code's.** `localcode` inherits
nothing; `-continue` takes the most recent chain in this repository and `-resume <id>` takes
a named one, which is the choice `claude` already offers and the one a developer already
knows. Inheriting by default would make a second task in a repository silently resume the
first.

**A chain is named by its first session's id**, so it matches what `claude` prints and what
sits under `~/.claude/projects/`. A session that ends says how to continue it, in the shape
`claude` uses, because the developer reading that line has just been told the other one.

**The flags are named after the ones that already exist.** Pi ships `--continue`, `--resume`,
`--session`, `--name` and `--fork`, and `claude` ships the first three; a developer moving
between them should not have to learn a third spelling. `-fork <id>` is taken from Pi and
earns its place: a chain that reached a decision worth trying twice is branched rather than
continued, and without it the only way to keep the earlier state is not to touch it.

**`localcode sessions` lists the chains in this repository** — id, when it last ran, and its
`Next`. A `-resume` that requires an id nobody recorded is a resume nobody uses.

**Automatic handover extends the current chain rather than starting one.** The distinction is
the whole model: handover is what happens inside an invocation, and choosing a chain is what
happens between them.

**Bounded, and the bound is a refusal rather than a silence.** `-max-sessions` caps the
chain, defaulting to a small number. A session that fills its context without advancing the
handoff would otherwise loop forever, and the honest failure is to stop and say which
handoff to read.

## Tasks

- [ ] a repository carries several chains, each with its own handoff and archive, and
      `localcode` starts a new one rather than inheriting
- [ ] `-continue` resumes the most recent chain, `-resume <id>` a named one, and
      `-fork <id>` branches one into a new chain
- [ ] `localcode sessions` lists the chains, and a session that ends says how to continue it
- [ ] a session's handoff is its own: the supervisor archives the inherited one, and
      `session-end.sh` writes for every session in a chain
- [ ] `localcode` ends a session on the first refusal recorded during it, and reports why
- [ ] the next session in the chain starts from that handoff, with the original instruction
      re-issued
- [ ] `SessionStart` injects the newest handoff and a bounded digest of the previous four
- [ ] two consecutive handoffs with the same `Next` stop the chain and name the file
- [ ] the chain is bounded by `-max-sessions`, and reaching it stops with the handoff named
- [ ] `-no-chain` runs a single session, and a session that ends normally never starts another
- [ ] the README says what the developer sees when a session hands over

## Open questions

- Whether the digest should carry each session's `Next` or its `Tried`. `Next` is what the
  session meant to do and `Tried` is what it did, and only a chain long enough to repeat
  itself will show which one prevents that.

## Log

- **Starting clean beat replaying and beat `--resume`.** Both re-ingest a conversation that
  had just failed to fit, and the measurement that motivated this feature is that a handoff
  costs 4,265 tokens where the conversation cost 37,837.
- **The staleness of an inherited handoff was found while planning this, not while using it.**
  It makes a chain freeze at its first handoff, and automatic handover is what turns that
  from an edge case into every session after the first.
- **One handoff per repository was wrong, and the correction is Claude Code's own model.** It
  assumed a repository holds one line of work, so a second task in the same checkout would
  have resumed the first and then overwritten its handoff. Chains are per invocation, the
  default is clean, and continuing is a choice with an id — which is what `claude` does and
  therefore what needs no explaining.
- **Pi was read before this was settled, and moved two things.** Its session model is the one
  being built here, so the flags take its spelling and `-fork` is taken outright. Its
  compaction defaults, read against the context this project serves, are evidence that
  compacting is not the answer somebody else already got right.
