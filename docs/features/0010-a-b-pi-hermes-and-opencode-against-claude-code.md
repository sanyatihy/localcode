---
id: 0010
title: A/B Pi, Hermes and OpenCode against Claude Code
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
model reads a file. Claude Code already works here and is the incumbent; the question is
not "which harness is good" but "does anything beat what is already in use".

## Non-goals

- **No custom harness.** The vision rules it out as a starting assumption; if every
  candidate fails, that outcome is written up before anyone proposes writing another.
- **No editor integration.** 0008's job. This compares agent loops at the same endpoint;
  where they are launched from is a separate question.
- **No serving-config tuning.** Both harnesses run against the same endpoint at the same
  config, or the comparison measures two things at once.
- **No model comparison.** 0007's axis.

## Design

Both candidates are model-agnostic and speak OpenAI-compatible endpoints, so both run
against 0001's server unchanged:

| Candidate | Shape | What it bets on |
|---|---|---|
| **Claude Code** (baseline) | Already in daily use, and 0008 shows it needs no proxy — `llama-server` serves the Anthropic Messages API natively | The incumbent. Anything that does not beat it is not worth switching to |
| Pi (`earendil-works/pi`) | Minimal core — Read, Write, Edit, Bash — with lazily loaded skills and a deliberately small system prompt | Context frugality: less preamble per turn leaves more budget for the actual repo |
| Hermes Agent (`NousResearch/hermes-agent`) | Persistent memory, skills learned from past trajectories, sandboxed execution | Accumulated context: it gets better at *this* repo over sessions |
| OpenCode (`opencode-ai/opencode`) | Go terminal agent, any OpenAI-compatible endpoint | Fit: same language as this project's own tooling, and a plain chat-completions client with no translation layer |

Claude Code is the **baseline, not a fourth arm**: it is measured once and the others are
reported as deltas against it. Switching cost is real, so a challenger that ties has lost.

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

- [ ] All three challengers are installed and each completes one task against 0001's endpoint, with exact invocations recorded
- [ ] Claude Code is scored first as the baseline, reusing 0008's configuration rather than a second setup
- [ ] Each is driven through 0002's adapters from a verified cold state, with any persistent memory reset between runs
- [ ] Tokens per completed task and turns-to-completion are recorded alongside the standard metrics
- [ ] Each is scored 3× on the full suite at the identical serving config, reported as a delta against the baseline
- [ ] The winner and margin are recorded in `docs/TECH.md`, with the context-per-turn figures that explain the result

## Open questions

- If Pi wins cold but Hermes' memory would plausibly win warm, what is the default?
  Leaning **Pi**, on the grounds that a cold-state result is the one this project can
  actually reproduce — with the warm question raised as a BACKLOG line rather than settled
  by argument here.
- How large a margin justifies leaving Claude Code? Leaning: **a clear win on tokens per
  completed task**, since that is the constraint this hardware actually imposes — a small
  success-rate edge is not worth relearning a tool.

## Log
- 2026-08-17 — added OpenCode, and made Claude Code the baseline rather than an
  omission: it is the incumbent, so the comparison is challengers-versus-it.
