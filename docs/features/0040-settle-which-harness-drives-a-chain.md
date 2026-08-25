---
id: 0040
title: Settle which harness drives a chain
status: Draft
created: 2026-08-25
shipped:
needs: 0039
---

## Problem

VISION says the harness question is answered by running the same tasks against the same
endpoint and scoring against the incumbent, and it is not answered. 0010 scored four
harnesses on five patch fixtures — Claude Code 14/15, Pi 12/15 — but that was one turn each
with no chain, no gate and no handoff, which is not the flow this project actually runs. The
one profiling comparison that exists points the other way: at a 12,288 wall Pi compacted five
times and finished where Claude Code died.

## Non-goals

- No third harness. Hermes is unattended-only on this machine and OpenCode scored below both;
  either re-entering is its own argument.
- No fixture chain. 0032 measured a generated fixture's calls at a third the cost of real
  source, so the small ceiling never binds in one and the comparison would not transfer.
- No change to either harness's bounds while the run is on. A comparison whose arms were
  tuned between them measures the tuning.

## Design

**Same instruction, same starting state, one token different.** `-harness` is the only thing
that moves, which is what 0039 exists to make true. Both arms carry the ceiling, the call
budget and the handoff, so what is compared is the harness and not the mechanism. The
starting state is a directory holding the month's two export files and no git repository at
all — the seed the two earlier instances of that project were built from — so each arm makes
its own first commit and the histories are comparable by construction rather than shared.

**Scored by what landed.** The arms are read out of the repositories they leave behind: the
features each shipped, the tests each wrote and whether they pass, and whether the
instruction's four clauses were answered. That reading happens after the run, because a chain
has no gate of its own — 0026 was dropped on the argument that verifying a deliverable is
kit's side of the line. Wall clock is the second column and not the first: 0034 measured the
two configurations within 3% per tool call, so a timing difference smaller than that is task
mix.

**The columns that decide are the ones a task mix cannot move.** Preamble per session,
tokens per tool call, sessions to finish, and how each session ended — on its ceiling, on its
call budget, or on its own work. 0032 and 0037 have those for Claude Code across eight
chains, so the incumbent arrives with a distribution rather than a single run.

**The instruction is the one that built this fixture twice already.** Four clauses in the
operator's own words: process one month's export, check the result against a second month,
visualise it, and keep working through features until none remain. It is open-ended on
purpose — a chain that plans its own features and works them until they run out is the flow
this project runs, and it is what the two earlier instances of the same project were driven
from, so the incumbent's eight chains and these arms are the same kind of work.

**One run an arm is not enough and the doc should say so.** Temperature is 0.7 on the served
config, so two chains do not do identical work; the spread between two Claude Code chains on
the same fixture was 46 tool calls against 35. Two runs an arm at minimum, and the finding is
the direction rather than the ratio.

## Tasks

- [ ] one instruction on a real repository is driven to completion on both harnesses, from
      the same starting commit, at least twice an arm
- [ ] the rows are recorded with what landed, what each session cost, and how it ended
- [ ] `docs/TECH.md` says which harness a chain runs on and what beat what

## Open questions

- **Whether Pi's compaction gets an arm of its own.** The two-arm run compares harnesses under
  this project's handoff mechanism, which is the question. A third arm — Pi compacting, as it
  ships — would say whether the mechanism is worth its cost at all, which is the question
  0023 left open and has never been measured. It costs a third of the run again.

## Log

- **The fixture and the instruction are settled: the private one, counts only.** Two fresh
  instances of a private work project are built from scratch, one an arm — the seed is a
  directory holding two months of export data and nothing else, byte-identical between the
  arms. Only counts are recorded here, as 0032 and 0037 already do, and neither the
  repository's name nor a file from it appears in this repository.
- **The design cited a gate that does not exist.** 0026 was dropped, so a chain runs no
  command between sessions and scores nothing by an exit code. What landed is read out of the
  two repositories after the run instead, which is a weaker reading than an exit code and is
  the one available.
- **"Same starting commit" was the wrong words for this seed.** The arms start before there
  is a git repository at all, so what is held identical is the directory, and each arm makes
  its own first commit.
