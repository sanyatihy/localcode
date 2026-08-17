---
id: 0010
title: A/B Pi against Hermes as the harness
status: Draft
created: 2026-08-17
shipped:
check:
checked:
review:
needs: 0001, 0002
related: 0008
---

## Problem

The harness decides how much of a local model's limited capability survives contact with
a real task: how tools are described, how much context each turn spends, how failures are
retried, when the loop stops. On a 32 GB machine where context is the binding constraint,
a harness that spends 8k tokens on preamble has taken a quarter of the budget before the
model reads a file. Two credible candidates exist and the choice should be measured.

## Non-goals

- **No custom harness.** The vision rules it out as a starting assumption; if both
  candidates fail, that outcome is written up before anyone proposes writing a third.
- **No Cursor.** Different shape — an editor routing through a cloud backend — and 0008's
  job, though this feature's winner is the local baseline 0008 measures against.
- **No serving-config tuning.** Both harnesses run against the same endpoint at the same
  config, or the comparison measures two things at once.
- **No model comparison.** 0007's axis.

## Design

Both candidates are model-agnostic and speak OpenAI-compatible endpoints, so both run
against 0001's server unchanged:

| Candidate | Shape | What it bets on |
|---|---|---|
| Pi (`earendil-works/pi`) | Minimal core — Read, Write, Edit, Bash — with lazily loaded skills and a deliberately small system prompt | Context frugality: less preamble per turn leaves more budget for the actual repo |
| Hermes Agent (`NousResearch/hermes-agent`) | Persistent memory, skills learned from past trajectories, sandboxed execution | Accumulated context: it gets better at *this* repo over sessions |

They are scored on 0002's suite at the identical serving config, on its existing metrics
plus two this comparison specifically needs:

- **Tokens consumed per completed task** — the direct measure of context frugality, and on
  this hardware the number most likely to decide it.
- **Turns to completion, and turns spent on failed tool calls** — a harness that recovers
  gracefully from a malformed call is worth more locally than one assuming a frontier model.

Hermes' memory feature is a **confound, not a bonus**: a harness that learns from previous
runs will score better on the second pass through a fixed suite for reasons that are not
quality. So each candidate is scored from a **cold state**, and any persistent memory is
reset between runs. Whether that memory helps over a real week is a separate question and
a BACKLOG line, not something this comparison can honestly measure.

## Tasks

- [ ] Both harnesses are installed and each completes one task against 0001's endpoint, with exact invocations recorded
- [ ] Both are driven through 0002's adapters from a verified cold state, with any persistent memory reset between runs
- [ ] Tokens per completed task and turns-to-completion are recorded alongside the standard metrics
- [ ] Each is scored 3× on the full suite at the identical serving config
- [ ] The winner and margin are recorded in `docs/TECH.md`, with the context-per-turn figures that explain the result

## Open questions

- If Pi wins cold but Hermes' memory would plausibly win warm, what is the default?
  Leaning **Pi**, on the grounds that a cold-state result is the one this project can
  actually reproduce — with the warm question raised as a BACKLOG line rather than settled
  by argument here.

## Log
