---
id: 0041
title: Stop a chain re-ingesting its preamble every session
status: Draft
created: 2026-08-25
shipped:
needs:
---

## Problem

A preamble is paid once per session and a session is what a handoff creates, so the term a
chain multiplies most is the one it never does any work with: 29% of everything the shipped
ceiling ingests and 46% at the smaller one. 0018 measured that a second session against a
warm server reused 83.5% of a 3,130-token preamble, and no chain has been measured with the
instrument that would say whether it gets that reuse or none of it.

## Non-goals

- No faster prefill. `--ubatch-size` was swept on both profiles and returns 0.5% on the one
  that motivated it; this is about ingesting less, not about ingesting quicker.
- No shorter preamble. The tool set and the briefing are what they are for reasons measured
  elsewhere; what is wrong is paying for the same tokens twice.
- No second cache. llama-server keeps prefixes and this is about whether it gets to use the
  one it already has.

## Design

**The claim this rests on is not yet measured, so the measurement is the first box.** The
number that looks like evidence — 4,165 tokens of preamble against 17,119 ingested — compares
the client's account of what it sent to the server's account of what it processed, and the two
answer different questions. Claude Code reports `cache_read_input_tokens` of 0 on every call
because llama-server does not fill that field, so the client cannot see server-side reuse at
all. `cmd/prefixlog` reads the server's own log and reports prompt, ingested, reused and how
the slot was chosen; that is the instrument, and 0018 is the precedent for pointing it at
real sessions.

**There are two mechanisms and they cost different amounts.** The first is that a chain's
sessions do not share a preamble byte for byte: the appended system prompt names the
session's own handoff path, `--add-dir` names its own directory, and the stalled briefing
varies with the count of still sessions. All three sit at the tail of the system prompt, so
what is lost is the tokens after the divergence — a few hundred. The second is slot
selection: llama-server picks a slot by longest-common-prefix similarity against a threshold,
and a chain's new session is a short prompt arriving at a slot holding the last session's
finished conversation. Below the threshold the slot is not reused at all and the whole
preamble is re-ingested. The measurement says which is happening, and only the second is
worth much.

**The fix follows the mechanism, and both candidates are cheap.** A stable preamble is a
handoff path passed by environment rather than named in the prompt, which the session already
reads for `LOCALCODE_HANDOFF_DIR`. A slot that is always reused is a serving flag, and this
build exposes `--cache-ram`, `--cache-idle-slots` and `--cache-reuse` while the similarity
threshold is compiled in. Which of those applies is not decidable before the first box.

**`--cache-ram` is the mechanism, it is on, and nothing here set it.** It is what saves a
finished slot's KV state to host memory so a later request with a matching prefix restores
rather than reprocesses — which is exactly what a chain's next session needs. It defaults to
8,192 MiB and no config in this repo names it, so every measurement ever taken here was taken
with 8 GiB of prompt cache enabled and unaccounted.

**And that 8 GiB is bought from the ceiling.** On unified memory the host prompt cache and
the Metal KV cache are the same physical resource, and the GPU's share of it is capped —
21,845 MiB on this machine, two thirds of the 32 GiB. So the reuse this feature is chasing is
in direct competition with the context a session may reach, and the documented advice for
Apple Silicon is `--cache-ram 0`, which would close this feature rather than complete it.
That makes the trade the design's first constraint: what the cache buys has to be measured in
tokens not re-ingested, against what it costs in context, and neither number exists yet.

**The cost side has a decision waiting on it already.** 0017 ruled the MTP draft head
inadmissible at 49,152 — the Metal command buffer failing
`kIOGPUCommandBufferCallbackErrorOutOfMemory` on the first prefill batch — and that screen
was taken with the 8 GiB prompt cache competing against it, because no config here names
`--cache-ram` and nothing recorded that it was on. The head is worth 1.26–1.57x on decode,
and decode is 73–81% of a chain's clock, so it is the largest speed lever this project has
and it was ruled out by the allocator failure that flag is documented to cause. Screening it
again with the cache off is how the cost of those 8 GiB is priced: if the head loads and
generates, the cache has been paying for preamble reuse with the biggest win available.

**Neither the screen nor the measurement can be run by a chain.** Both stop what is serving
8081 and start something else on it, and a session driven by `localcode` is answered by that
server — so a chain running either would be killing the model taking its own turns. This
feature is operator work or frontier-tier work, not local-tier work, and that is a property
of anything that changes what is served rather than a limitation of the model.

**What must hold afterwards is a number, not an argument.** The same instruction driven on
the same repository, before and after, with the server's own reused figure per session read
the same way both times.

## Tasks

- [x] `serve.sh` passes `CACHE_RAM` when a config names it, the way it already passes
      `BATCH_SIZE`, and no committed config's behaviour changes
- [x] `config/mtp-49k-nocache.env` screens admissible or not with the prompt cache off, and
      `docs/TECH.md` says whether the draft head is adopted at the shipped context
- [x] what a chain's sessions actually reuse is measured per request, from the server's own
      log, across a chain of at least four sessions, with the prompt cache's size on the row
- [ ] the preamble a chain sends is identical from session to session, or the measurement
      says it does not matter and this box is dropped
- [ ] the reuse a chain gets is measured again against the same instruction, and `docs/TECH.md`
      carries what changed

## Open questions

- **Whether the ceiling is 83.5% or higher.** 0018 measured 516 tokens of a 3,130-token
  preamble differing between two runs of the *same* command, so something in Claude Code's own
  preamble varies that this cannot reach. If that is the floor, a stable preamble buys the
  difference between what a chain gets now and 83.5%, not between what it gets and nothing.
- **Whether the slot threshold is reachable at all.** The similarity threshold is not a flag
  in build 10450, so if slot rejection is the mechanism the lever may be `--parallel` or
  `--cache-idle-slots` rather than the threshold itself. Leaning towards measuring before
  reading any more of llama.cpp.

## Log
- 2026-08-25 — the feature widened from getting the reuse to pricing it. `--cache-ram` is
  8,192 MiB by default and no config here names it, so the 8 GiB it takes from the pool the
  KV cache is capped in has been unaccounted in every measurement this repo holds — including
  0017's screen, which ruled the draft head inadmissible at 49,152 on exactly the allocator
  failure that flag is documented to cause. The screen is now a box, because the open question
  about whether the cache pays for itself cannot be answered from the benefit alone.
- 2026-08-25 — the screen goes first. It is twenty minutes against the hours a four-session
  chain costs, it needs nothing the reuse measurement produces, and what it settles — whether
  the largest speed lever this project has was ruled out by an unaccounted flag — changes
  whether the reuse is worth chasing at all.
- 2026-08-25 — the screen came back refused, and the cost side of this feature's trade is
  gone. `config/mtp-49k-nocache.env` fails exactly as `config/mtp-49k.env` does —
  `kIOGPUCommandBufferCallbackErrorOutOfMemory` at `n_batch = 2048`, 500 within a second — so
  the 8 GiB prompt cache was not what ruled the draft head out at 49,152 and turning it off
  does not buy the head back. The mechanism is that `--cache-ram` bounds a cache filled
  lazily rather than reserving memory at load: the two screens peak 12 MB apart. What that
  settles for the boxes below is that the cache may be measured on its benefit alone, and the
  open question about what to do when both sides of the trade pay is answered — only one side
  ever did.
- 2026-08-25 — measured, and the design had the mechanism the wrong way round. A four-session
  chain reuses 88.4% overall and **0.02% of its session-opening requests**: each one ingests
  its whole preamble, 13,555 tokens over three handoffs, 43.5% of everything the chain
  ingested and 12.1% of its wall. Slot rejection is ruled out — all three were selected by LCP
  similarity at f_sim 0.638–0.657 against a 0.100 threshold — so the second mechanism this
  design weighed is not the one operating, and the first is worse than it was written to be.
  The divergence is at the **head** of the preamble, not its tail, so what is lost is not "the
  tokens after the divergence — a few hundred" but all of them. The next box is therefore not
  droppable: making the preamble identical is the only lever the measurement leaves.
