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
- 2026-08-20 — **the last box drops the fix, because there is nothing left to fix.** It read
  "the fix that cause implies is measured … served by `config/agent.env` if it wins", and the
  cause is a prefix the server has never seen. Nothing serves that away: a first request has
  nothing to reuse by construction. What replaces it is recording what was established, which
  is the part of that box worth keeping.

  **This ships rather than being dropped.** A drop retires a feature that should not have
  been built; this one asked a real question about a real cost and answered it in numbers a
  later feature can read. What it does not produce is a change to the serving config, and
  that is a result rather than a failure — 0006 and 0010 each ship one. The title is the
  casualty: it names a mechanism this feature disproved, and it stands because renaming the
  doc renames the branch that claims it. Retiring it instead is a reviewer's call, and the
  evidence is committed either way.
- 2026-08-20 — **the turns that ingest from zero are the ones with nothing to reuse, and
  there is one.** Two sessions back to back against one server: 22 requests, 286,148 prompt
  tokens, 34,386 ingested, 88.0% reused. The only from-zero ingest is the first request of
  the first session, against a server that had never served that prefix.

  A fresh session is not one of them. The second session's first request sent the same
  3,130-token preamble and ingested 516 of it — 83.5% reused, 4.5 s against the first
  session's 27.6 s. So about 516 tokens of Claude Code's preamble differ between two
  otherwise identical runs, and they sit at its tail. **That settles what `docs/TECH.md`
  leaves open** where it records Claude Code re-ingesting its preamble on every run and says what
  changes is not established.

  It reproduces with the probe, which is what makes it a property of the server rather than
  of one harness: turn 1 of every condition ingests its whole prompt and every later turn
  ingests only what is new. The scripted conversation and the real session agree, and the
  deepest real request — 21,813 tokens, the size the problem statement reports — reused 97%
  of itself.
- 2026-08-20 — **a real session on this config does not re-read itself either.** Thirteen
  requests of a `claude -p` session over a copy of this repo: 145,180 prompt tokens, 16,308
  ingested, 88.8% reused. One request ingested from zero, and it is the first, which has
  nothing to reuse by construction. Twelve of the thirteen were chosen by LCP similarity
  and hit between 79% and 99%.

  **The session made no calls beside the conversation at all** — its prompts grow
  monotonically from 3,130 to 18,208 — because `CLAUDE_CODE_DISABLE_TERMINAL_TITLE` in
  `harness/claude-code/claude-code.env` suppresses them for `claude -p`. The traffic this
  feature is named after does not occur under the environment the repo ships.

  What is left that could ingest 22,770 tokens from zero is a session's *first* request.
  It is cold by construction, and at ~90 tok/s it costs the four minutes the observation
  reports. 0016 runs bounded sessions and starts a fresh one each time, so that shape pays
  it once per session rather than once. The next box tests it: a second session against the
  same server either reuses the first's preamble or does not.
- 2026-08-20 — **boxes 2 and 3 are replaced, and the premise with them.** `PARALLEL=2` and
  a second small model both stop side traffic taking the slot, which the box below measured
  as costing the conversation nothing — measuring them would price a fix for a fault this
  config does not have. What replaces them is finding the fault, on a real session, since
  a scripted one does not exhibit it. The `## Problem` paragraph goes too: the observation
  stands and its explanation does not.

  **The title now names a mechanism this feature disproved**, and it is left standing for
  one box rather than corrected twice. If the cause turns out to have nothing to do with
  traffic beside the conversation, this doc is a `kit drop` and its successor a redraft;
  if it does, the title is close enough to fix in place. The next box decides which, and
  a wrong title is cheaper to carry until then than a plan round spent guessing.
- 2026-08-20 — **the mechanism in the problem statement does not reproduce, at any depth
  this config serves.** The conversation ingests its 28,121 tokens exactly once whether or
  not a 5,531-token call sits between every turn, and the per-turn figures are identical to
  the token: 8,034, 5,027, then 5,020 a turn. At the ceiling it is 43,181 against 43,188
  over eight turns. It holds for both shapes of traffic beside a conversation — one sharing
  no prefix with it, and one opening with its system prompt, which the server does serve
  partly from cache (3,016 tokens) without touching the conversation's.

  The reason is a server feature the design table does not account for: this build keeps
  prefixes it evicts from a slot in host RAM and restores them, bounded by `--cache-ram`,
  8192 MiB by default and set in no config here. This model's q8_0 KV costs 138.1 KiB a
  token — 65 layers, 4 KV heads, 256 wide for K and V — so that budget holds ~60,700 of
  them. It is more than this config's whole 49,152 window and a call beside it. A small
  call borrows the slot rather than taking the prefix. **That also answers the open
  question this feature opened with:** a slot's KV is its own, but reuse is not bounded
  by the slot.

  `selected slot by LRU` is not the evidence the problem statement reads it as. All 15
  requests of the ceiling run logged it and none logged `LCP similarity`, including the
  turn that went on to reuse 38,175 tokens.

  What the interleaving does cost is the calls themselves — 215.9 s over four at 28k,
  375.9 s over seven at the ceiling. That is a real charge on a session, and neither
  candidate in the design table removes it.
