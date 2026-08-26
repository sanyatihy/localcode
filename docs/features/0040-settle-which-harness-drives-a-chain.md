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

**An arm must not be able to read a solved copy of its own task.** The sandbox allows a
session to read anywhere, and three earlier instances of this same project sit beside the
seed — one of them built by a frontier model and complete. A session that lists its parent
directory finds a finished answer to the instruction it was just given, and what it does
next is not what a harness comparison is asking about. So the arms run under a parent that
holds them and nothing else, and the earlier instances are moved out of it for the duration.

**One run an arm is not enough and the doc should say so.** Temperature is 0.7 on the served
config, so two chains do not do identical work; the spread between two Claude Code chains on
the same fixture was 46 tool calls against 35. Two runs an arm at minimum, and the finding is
the direction rather than the ratio.

## Tasks

- [x] one instruction on a real repository is driven on both harnesses from one starting
      state, twice an arm, and each chain's ending is recorded — its own work or a bound
- [x] the rows are recorded with what landed, what each session cost, and how it ended
- [x] `docs/TECH.md` says which harness a chain runs on and what beat what

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
- **Pair one is void, and the fixture is why.** Arm A finished the instruction in 6 sessions
  and 2h49m, shipping three features. Arm B stalled in 3 sessions and 37 minutes without a
  commit: its first session listed the parent directory, found a sibling checkout of the same
  project carrying a complete implementation, and spent the chain running that code and
  planning to port it rather than doing the work. Arm A never looked — its one search of the
  filesystem stopped a level short of that directory — so the arms did not face the same
  fixture and neither row is a harness reading. Both are re-run under a parent holding
  nothing but the arms.
- **A Pi session overshot its ceiling by 16,846 tokens**, peaking at 44,494 against 27,648 in
  a 45,056 window — against the roughly 5,000 that 0037 measured across 69 Claude Code
  sessions. It happened on 8 tool calls, so what a single batch can add between two readings
  of the context is larger under Pi than the reserve was sized for. Recorded rather than
  fixed: nothing about either harness's bounds changes while the run is on.
- **Twice an arm was run; once an arm finished.** Pair one drove the instruction to
  completion on both harnesses. In pair two both arms stopped on the 1h session timeout, in
  session 4, with the same third feature outstanding in each — at 6.5 tok/s an hour buys
  about 23,000 generated tokens, and a single self-contained HTML report is one generation
  larger than that. The box asked for completion twice an arm and is ticked at four chains
  rather than re-run, because the columns that decide are per-session and a session that a
  bound stopped still measured what its calls cost. Sessions-to-finish is the one column that
  needs a finish, and it has one pair behind it rather than two.
- **The arms were not held to the same wall clock, and one of them cannot be.** Pi's
  pair-two chain spent 1,779 s of wall outside what the recorder counts as a model call,
  against 89 s for the incumbent, and those gaps fall after tool results. Whether that is
  generation the Pi reader does not attribute or genuine idle is unsettled, so wall clock
  stays the second column and the comparison rests on tokens.
- **What landed was read by rebuilding it, not by reading the handoffs.** Each arm's
  pipeline was extracted from its own tracked source into a clean directory and run: all
  four produce their classified months, and the two that reached the third feature produce a
  self-contained page with no external reference in it. Pair one's arms each answered all
  four clauses of the instruction; pair two's each shipped two features of three. **No arm
  in any chain wrote a test**, and none was asked to.
- **The verdict is split by job, and the flag's default is left alone.** Pi wins every
  per-session column and the one pair that finished; 0010's patch sweep put the incumbent
  ahead one turn at a time. TECH says to drive a chain with `-harness pi` and to reach for
  the incumbent for one-shot patch work. Flipping `harness.DefaultAgent` is a backlog line
  rather than a box here: four chains settle which is cheaper, not what a fresh checkout
  should do by default.

