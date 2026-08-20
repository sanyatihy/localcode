---
id: 0018
title: Stop small calls evicting the conversation's prefix
status: Draft
created: 2026-08-20
shipped:
check:
checked:
review:
needs:
related: 0003, 0008, 0016
---

## Problem

A local session spends most of its wall clock re-reading itself. `config/agent.env` serves
`PARALLEL=1`, so llama.cpp holds one cached prefix; Claude Code interleaves small calls with
the conversation — `ANTHROPIC_DEFAULT_HAIKU_MODEL` points at the same GGUF, so titling and
similar land on the same endpoint — and each one evicts the conversation from the slot. The
next real turn finds nothing to reuse and ingests everything again.

Measured while a builder worked a task box: a 22,770-token turn re-ingested from zero at
~90 tok/s, **four minutes before its first generated token**, with the server log showing
`selected slot by LRU` where a hit would say `LCP similarity`, and 5,530- and 5,599-token
requests either side of it. At 30k that cost grows to five minutes and it is paid most
turns.

This is a larger drag on a local session than compaction was. 0016 removed roughly eight
minutes per compaction; this is four minutes per *turn*.

## Non-goals

- **No harness change.** Which calls Claude Code makes is its business; this is about what
  the server does with them.
- **No new serving config for the scorer.** `config/tuned.env` stays at one slot: tier-1
  runs are single requests with no conversation to preserve.
- **Not a cache-size question.** The prefix is evicted, not outgrown — the slot is holding
  a 5k prompt where a 25k one was.

## Design

Two candidate fixes, and the point of the feature is to measure rather than to choose:

| | what it does | what it costs |
|---|---|---|
| **Two slots** (`PARALLEL=2`) | a small call takes the other slot and leaves the conversation's prefix alone | KV is split across slots, so the served context per slot halves unless `CTX_SIZE` doubles — and 0003 measured ingest, not memory, as what binds at 64k |
| **A second model for the small calls** | `ANTHROPIC_DEFAULT_HAIKU_MODEL` points at a 0.6B served on another port, so those calls never touch this endpoint | a second process and its memory, against a model that answers in a second |

**The measurement is the same either way**: drive one fixed conversation through a session
and record, per turn, what the server ingested against what it served from cache. The
number that matters is the share of turns that hit the prefix, and the wall clock those
turns cost.

**The baseline already exists**: one slot, one model, both calls on the same endpoint, which
is what every session in 0011 and 0016 ran on.

`/props` reports `n_ctx` per slot and `/slots` reports `n_prompt_tokens_cache`, so the
instrument is already in the server; 0010's counters are read the same way.

## Tasks

- [x] The premise is tested deliberately: a fixed conversation, a small call interleaved, and the cache-hit rate recorded per turn against the baseline config
- [ ] `PARALLEL=2` is measured on the same conversation — cache hits, served context per slot, and whether the desktop verdict at that context still holds
- [ ] A second small model on its own port is measured the same way, with its memory recorded beside the model's
- [ ] The winner is served by `config/agent.env` if it wins, and `docs/TECH.md` records the per-turn ingest cost before and after

## Open questions

## Log
- 2026-08-20 — **the mechanism in the problem statement does not reproduce, at any depth
  this config serves.** The conversation ingests its 28,121 tokens exactly once whether or
  not a 5,531-token call sits between every turn, and the per-turn figures are identical to
  the token: 8,034, 5,027, then 5,020 a turn. At the ceiling it is 43,181 against 43,188
  over eight turns. It holds for both shapes of traffic beside a conversation — one sharing
  no prefix with it, and one opening with its system prompt, which the server does serve
  partly from cache (3,016 tokens) without touching the conversation's.

  The reason is a server feature the design table does not account for: this build keeps
  prefixes it evicts from a slot in host RAM and restores them, bounded by `--cache-ram`,
  8192 MiB by default and set in no config here. At 138.1 KiB per token of q8_0 KV — 65
  layers, 4 KV heads, 256 wide for K and V — that budget is ~60,700 tokens, more than this
  config's 49,152 window and a call beside it, so a small call borrows the slot rather than
  taking the prefix. **That also answers the open question this feature opened with:** a
  slot's KV is its own, but reuse is not bounded by the slot.

  `selected slot by LRU` is not the evidence the problem statement reads it as. All 15
  requests of the ceiling run logged it and none logged `LCP similarity`, including the
  turn that went on to reuse 38,175 tokens.

  What the interleaving does cost is the calls themselves — 215.9 s over four at 28k,
  375.9 s over seven at the ceiling. That is a real charge on a session, and neither
  candidate in the design table removes it.
