---
id: 0018
title: Stop small calls evicting the conversation's prefix
status: Shipped
created: 2026-08-20
shipped: 2026-08-20
check:
checked:
review:
needs:
related: 0003, 0008, 0016
---

## Problem

A local session spends much of its wall clock re-reading itself. Measured while a builder
worked a task box: a 22,770-token turn re-ingested from zero at ~90 tok/s, **four minutes
before its first generated token**, with 5,530- and 5,599-token requests either side of it.
0011 records the same shape from the other end: at 49,152 the conversation is re-ingested
most turns. At 30k that cost is five minutes.

This is a larger drag on a local session than compaction was. 0016 removed roughly eight
minutes per compaction; this is four minutes per *turn*.

**What causes it is not established.** The feature was drafted against one explanation —
that `PARALLEL=1` leaves llama.cpp one cached prefix, and that a small call on the same
endpoint takes the slot, so the next real turn finds nothing to reuse. Box 1 measured it
and it is not what happens. The conversation keeps its prefix through such calls at every
depth this config serves, because the server restores an evicted prefix from host RAM.
The cost is real; its cause is open.

## Non-goals

- **No harness change.** Which calls Claude Code makes is its business; this is about what
  the server does with them.
- **No new serving config for the scorer.** `config/tuned.env` stays at one slot: tier-1
  runs are single requests with no conversation to preserve.
- **Not a cache-size question.** The prefix is evicted, not outgrown — the slot is holding
  a 5k prompt where a 25k one was.

## Design

**Find the fault before pricing a fix.** The two candidates this was drafted with — a
second slot, and a second model for the small calls — both stop traffic beside the
conversation from taking the slot. Box 1 measured that as costing the conversation
nothing, so both would buy a fix for a fault this config does not have.

**A scripted conversation cannot produce the failure**, which box 1 established at two
depths and against both shapes of side traffic. So the subject has to be a real session,
and the instrument has to be one that works on traffic nobody wrote: the server's own log
prints, per request, how it chose a slot and how many tokens it processed against how many
it reused. That is the only place a real session's per-request accounting survives.

**What box 1 built is the control.** `cmd/prefixprobe` replays a fixed conversation whose
per-turn ingest is known to the token. Any explanation the log offers can be put back to
it and reproduced deliberately. One that cannot be is a finding of its own: it says the
cost lives in something a script does not stage.

## Tasks

- [x] The premise is tested deliberately: a fixed conversation, a small call interleaved, and the cache-hit rate recorded per turn against the baseline config
- [x] Every request of a real session on `config/agent.env` is accounted from the server's own log — prompt, ingested, reused, and how the slot was chosen — so the turns that ingest from zero are identified rather than inferred
- [x] What those turns have in common is named, and either reproduced with `prefixprobe` or shown to be beyond what a scripted conversation can stage
- [x] `docs/TECH.md` records what a session's prompt actually costs on this config, what the server does to make it that, and that no serving change follows

## Open questions

## Log

- 2026-08-20 — **the mechanism in the problem statement does not reproduce, at any depth this
  config serves**, so the premise this feature was drafted on is falsified. A conversation
  ingests each token exactly once whether or not a small call sits between every turn, for
  both shapes of side traffic — one sharing no prefix, one opening with the conversation's
  system prompt.

  The reason is a server feature the design table does not account for: this build keeps
  prefixes it evicts from a slot in host RAM and restores them, bounded by `--cache-ram` and
  set in no config here. A small call borrows the slot rather than taking the prefix. **That
  also answers the open question this feature opened with** — a slot's KV is its own, but
  reuse is not bounded by the slot. And `selected slot by LRU` is not the evidence the problem
  statement reads it as: every request of the ceiling run logged it, including one that went
  on to reuse 38,175 tokens.
- 2026-08-20 — **boxes 2 and 3 are replaced, and the `## Problem` paragraph with them.**
  `PARALLEL=2` and a second small model both stop side traffic taking the slot, which costs
  the conversation nothing — measuring them would price a fix for a fault this config does not
  have. What replaces them is finding the fault on a real session, since a scripted one does
  not exhibit it.

  **The title now names a mechanism this feature disproved**, and is left standing for one box
  rather than corrected twice: if the cause has nothing to do with traffic beside the
  conversation this doc is a `kit drop`, and if it does the title is close enough to fix in
  place. A wrong title is cheaper to carry until the next box decides than a plan round spent
  guessing.
- 2026-08-20 — **the traffic this feature is named after does not occur under the environment
  the repo ships.** `CLAUDE_CODE_DISABLE_TERMINAL_TITLE` in `harness/claude-code/claude-code.env`
  suppresses it for `claude -p`, and a real session's prompts grow monotonically. What is left
  that could ingest from zero is a session's *first* request, cold by construction — and 0016
  runs bounded sessions, so that shape pays it once per session. The next box tests whether a
  second session against the same server reuses the first's preamble.
- 2026-08-20 — **that settles what `docs/TECH.md` left open** where it recorded Claude Code
  re-ingesting its preamble on every run without establishing what changes. A fresh session is
  not a from-zero ingest: 516 tokens of the preamble differ between two otherwise identical
  runs, at its tail. It reproduces with the probe, which is what makes it a property of the
  server rather than of one harness.
- 2026-08-20 — **the last box drops the fix, because there is nothing left to fix.** It asked
  for the fix the cause implies, served by `config/agent.env` if it wins; the cause is a prefix
  the server has never seen, and nothing serves that away. Recording what was established
  replaces it.

  **This ships rather than being dropped.** A drop retires a feature that should not have been
  built; this one asked a real question and answered it in numbers a later feature can read.
  Producing no change to the serving config is a result rather than a failure. The title is
  the casualty and it stands, because renaming the doc renames the branch that claims it;
  retiring it instead is a reviewer's call, and the evidence is committed either way.
