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

**Same instruction, same repository, same starting commit, one token different.** `-harness`
is the only thing that moves, which is what 0039 exists to make true. Both arms carry the
gate, the ceiling and the handoff, so what is compared is the harness and not the mechanism.

**Scored by what landed.** 0026 gives a chain an external success test that is a command and
an exit code, so the arms are read by the repository rather than by what a session claimed.
Wall clock is the second column and not the first: 0034 measured the two configurations
within 3% per tool call, so a timing difference smaller than that is task mix.

**The columns that decide are the ones a task mix cannot move.** Preamble per session,
tokens per tool call, sessions to finish, and how each session ended — on its ceiling, on its
call budget, or on its own work. 0032 and 0037 have those for Claude Code across eight
chains, so the incumbent arrives with a distribution rather than a single run.

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

- **Which repository and which instruction.** A private work repository is what 0032 used and
  is what makes the calls cost what real calls cost; only counts may be recorded from it.
  This project's own backlog is the alternative and is public, but a chain working on the repo
  that scores it is a confound. Leaning towards the private one, recording counts only, as
  0032 and 0037 already do.
- **Whether Pi's compaction gets an arm of its own.** The two-arm run compares harnesses under
  this project's handoff mechanism, which is the question. A third arm — Pi compacting, as it
  ships — would say whether the mechanism is worth its cost at all, which is the question
  0023 left open and has never been measured. It costs a third of the run again.

## Log
