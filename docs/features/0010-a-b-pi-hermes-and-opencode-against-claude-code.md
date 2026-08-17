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
| OpenCode (`anomalyco/opencode`) | The most actively developed of the three; any OpenAI-compatible endpoint | Momentum and breadth of provider support, and a plain chat-completions client with no translation layer |

Claude Code is the **baseline, not a fourth arm**: it is measured once and the others are
reported as deltas against it. Switching cost is real, so a challenger that ties on quality
has lost — **except on one axis where the baseline is known to fail**.

**Offline capability is a scored criterion, not a footnote.** 0008 establishes that Claude
Code contacts Anthropic for OAuth refresh and feature flags whatever `ANTHROPIC_BASE_URL`
points at, so it delivers local inference but not an offline harness. Each challenger is
therefore tested with the network disabled, and whether it works at all is recorded
alongside its scores. A challenger that runs fully offline holds something the incumbent
cannot, and that is the vision's "at least one path is genuinely offline" outcome — so it
can win on this axis even while losing on quality, and the write-up must say which it won on.

They are scored on 0002's suite at the identical serving config, **per profile**, on its
existing metrics plus three this comparison specifically needs:

- **Tokens consumed per completed task** — the direct measure of context frugality, and on
  this hardware the number most likely to decide it.
- **Prompt-cache preservation** — whether the harness keeps a stable prefix across turns, or
  rewrites history (compaction, reordering, re-summarising) and forces a re-ingest. Measured
  at 91.8 tok/s cold, a full 32k re-ingest costs about six minutes, so a harness that
  invalidates the cache mid-task is not slightly worse but unusable for the attended
  profile. Measured as cache-hit rate per turn, not inferred from documentation.
- **Turns to completion, and turns spent on failed tool calls** — a harness that recovers
  gracefully from a malformed call is worth more locally than one assuming a frontier model.

Hermes' memory feature is a **confound, not a bonus**: a harness that learns from previous
runs will score better on the second pass through a fixed suite for reasons that are not
quality. So each candidate is scored from a **cold state**, and any persistent memory is
reset between runs. Whether that memory helps over a real week is a separate question and
a BACKLOG line, not something this comparison can honestly measure.

## Tasks

- [ ] The identifier and current home of each challenger is confirmed before install — OpenCode in particular has an archived predecessor under a different owner
- [ ] All three challengers are installed and each completes one task against 0001's endpoint, with exact invocations recorded
- [ ] Claude Code is scored first as the baseline, reusing 0008's configuration rather than a second setup
- [ ] Each is driven through 0002's adapters from a verified cold state, with any persistent memory reset between runs
- [ ] Tokens per completed task and turns-to-completion are recorded alongside the standard metrics
- [ ] Each is scored 3× on the full suite at the identical serving config, reported as a delta against the baseline
- [ ] Each challenger is run with the network disabled, and whether it completes a task offline is recorded as a first-class result
- [ ] The winner and margin are recorded in `docs/TECH.md`, with the context-per-turn figures that explain the result, and with the offline result stated separately from the quality result

## Open questions

- If Pi wins cold but Hermes' memory would plausibly win warm, what is the default?
  Leaning **Pi**, on the grounds that a cold-state result is the one this project can
  actually reproduce — with the warm question raised as a BACKLOG line rather than settled
  by argument here.
- How large a margin justifies leaving Claude Code? Leaning: **a clear win on tokens per
  completed task**, since that is the constraint this hardware actually imposes — a small
  success-rate edge is not worth relearning a tool.
- If the best challenger is the only offline-capable option but loses on quality, is the
  answer one harness or two? Leaning **two, kept deliberately**: the incumbent for ordinary
  work and an offline harness for when there is no network. Naming that as a supported
  outcome now stops the comparison being forced into a single winner it does not have.

## Log
- 2026-08-17 — added OpenCode, and made Claude Code the baseline rather than an
  omission: it is the incumbent, so the comparison is challengers-versus-it.
- 2026-08-17 — corrected the OpenCode identifier to `anomalyco/opencode`; the previously
  cited `opencode-ai/opencode` is an archived predecessor, last pushed 2025-09-18.
- 2026-08-17 — added offline capability as a scored criterion after 0008 established the
  baseline cannot have it.
