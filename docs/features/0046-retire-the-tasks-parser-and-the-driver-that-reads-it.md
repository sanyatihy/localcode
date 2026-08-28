---
id: 0046
title: Retire the Tasks parser and the driver that reads it
status: Shipped
created: 2026-08-27
shipped: 2026-08-28
needs:
---

## Problem

`internal/handoff` parses `kit`'s `## Tasks` format and `cmd/handoff` drives "the topmost
unticked box" from it — the one place this repo re-implements `kit`, which VISION names as
the wrong side of the boundary. Its `BACKLOG.md` entry has waited on 0026 shipping, and
0026 was dropped for the reason that makes the parser wrong.

## Non-goals

- **Not removing the transcript readers.** `ReadSession`, `Requests` and `Request` are
  what `chain.Cost` and `localcode account` read a finished session with. Only the doc
  parser and the driver over it go.
- **Not replacing `cmd/handoff` with anything.** `localcode "Do the topmost unticked box
  in <doc>, and only that one"` is the same instruction through the driver that has the
  sandbox, the budget, the gate, the chain endings and `-resume`.

## Design

**The trigger cannot fire, and its failure is the argument.** The entry says *promote when
0026 ships*. 0026 is `Dropped`, and its log says why: "verification of the deliverable is
kit's side of the line. localcode owns runtime — window, ceiling, handoff, sandbox — and
nothing about what a session delivered." The parser is the same claim in code. Waiting on a
condition that was closed by agreeing with you is waiting for nothing.

**What goes has exactly one consumer.** `Boxes`, `Topmost`, `Ticked`, `Box` and `Refusals`
are reachable from `cmd/handoff` and from nothing else; `ReadSession` is shared with
`internal/chain`. So the cut is 306 lines of `cmd/handoff`, its 93-line test, 84 lines of
`internal/handoff` and four of its nine tests — and no other call site moves.

**`cmd/handoff` is a weaker `cmd/localcode`.** It runs `claude` directly with no sandbox,
no budget, no gate, no chain record and no resume, and its one unique property is checking
the doc rather than the model's word. That property is what 0026 put on kit's side.

**What remains is a transcript reader called `handoff`.** After the cut the package holds
nothing to do with `HANDOFF.md`, which `internal/chain` owns — two meanings of one word,
and the surviving one is the smaller. Renaming it is the last box rather than the first,
because a rename before the cut moves lines that are about to be deleted.

## Tasks

- [x] `cmd/handoff` and the `## Tasks` parser are gone, and `make check` passes without
      them
- [x] Every doc that pointed at them points at `localcode` instead, and the `BACKLOG.md`
      entry that waited on 0026 is retired with its reason
- [x] What is left of `internal/handoff` is named for what it reads, and nothing else
      changes with it

## Log
- 2026-08-28 — the `BACKLOG.md` entry that waited on 0026 was already gone: the plan round
  that wrote this doc (b0c16d8) retired it as promoted to 0046, which is the reason this box
  would have recorded. What box 2 changed is the three docs that still named `cmd/handoff`.
- 2026-08-28 — the survivor is `internal/transcript`. The name shadows the parameter three
  callers gave a transcript path, so those three are `path` now; nothing else moved.
