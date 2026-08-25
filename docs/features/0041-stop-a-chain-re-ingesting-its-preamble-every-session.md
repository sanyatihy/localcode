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

**What must hold afterwards is a number, not an argument.** The same instruction driven on
the same repository, before and after, with the server's own reused figure per session read
the same way both times.

## Tasks

- [ ] what a chain's sessions actually reuse is measured per request, from the server's own
      log, across a chain of at least four sessions
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
