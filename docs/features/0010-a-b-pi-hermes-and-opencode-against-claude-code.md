---
id: 0010
title: A/B Pi, Hermes and OpenCode against Claude Code
status: Shipped
created: 2026-08-17
shipped: 2026-08-19
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

**Hermes cannot be scored attended on this machine, and that is a constraint rather than a
result.** It refuses any context under 64,000 tokens. 0014 measured the attended ceiling at
**57,344** — 65,536 completes every request while leaving the desktop unusable, and the
compositor stalls for the whole run. Hermes' floor therefore sits inside the inadmissible
band with no overlap: the smallest context it will accept is one the machine cannot carry
while someone is using it.

The floor is a hard check in Hermes' own code, not a setting — `context_length` in
`~/.hermes/config.yaml` selects what it asks for, and anything under 64,000 is refused before
a request is made. Lowering it means patching Hermes, and a patched Hermes is a different
harness than the one under comparison; if that is ever done it is scored under its own name.

So Hermes is scored **unattended only**, and its numbers are not comparable to a challenger
scored attended without saying so. Scoring it attended anyway would produce a quality figure
for a configuration nobody can use, which is worse than a gap in the table. Whether it wins
unattended is still a real and useful question, and the grind profile is where it gets asked.

## Tasks

- [x] Hermes is scored under the unattended profile only, with its 64,000-token floor against 0014's 57,344 attended ceiling stated in the results table rather than left to be inferred from a missing row
- [x] A tier-2 row carries the context the server reported serving, and a harness whose floor that server does not meet is recorded as inadmissible rather than driven
- [x] The identifier and current home of each challenger is confirmed before install — OpenCode in particular has an archived predecessor under a different owner
- [x] All three challengers are installed and each completes one task against 0001's endpoint, with exact invocations recorded
- [x] The unseen test is staged only once the harness has stopped, so a harness that reads or runs the workdir cannot score against it
- [x] Claude Code is scored first as the baseline, reusing 0008's configuration rather than a second setup
- [x] Each is driven through 0002's adapters from a verified cold state, with any persistent memory reset between runs
- [x] Tokens per completed task and turns-to-completion are recorded alongside the standard metrics
- [x] A tier-2 run is bounded by a budget, and a harness still working when it expires is recorded as over budget rather than as a broken adapter
- [x] Each is scored 3× on the full suite at the identical serving config, reported as a delta against the baseline
- [x] Each challenger is run with the network disabled, and whether it completes a task offline is recorded as a first-class result
- [x] The winner and margin are recorded in `docs/TECH.md`, with the context-per-turn figures that explain the result, and with the offline result stated separately from the quality result

## Log

- 2026-08-17 — added OpenCode, and made Claude Code the baseline rather than an omission: it is
  the incumbent, so the comparison is challengers-versus-it.
- 2026-08-17 — corrected the OpenCode identifier to `anomalyco/opencode`; the previously cited
  `opencode-ai/opencode` is an archived predecessor.
- 2026-08-17 — added offline capability as a scored criterion after 0008 established the
  baseline cannot have it. *0008 later reversed that, and the axis separates nothing.*
- 2026-08-18 — inherited from 0014: Hermes' floor is above the measured attended ceiling, so it
  is admissible only for unattended work here. Recorded before the comparison runs, so it is a
  stated constraint on the design rather than a bad result discovered at scoring time.
- 2026-08-19 — added a box above the sweep for bounding a run. Hermes spent 45 minutes and 90
  turns on one fixture before the adapter's timeout stopped it, recorded as a broken adapter —
  which it was not. A looping harness is a result about the harness, and an unbounded one makes
  a 3× sweep unschedulable. The sweep restarts once runs are bounded, and the partial rows
  already taken keep their own conditions.
- 2026-08-19 — added a box for staging the unseen test after the run. `RunTier2` wrote it into
  the scratch module before the harness started, and the candidates differ in how much of the
  workdir they read, which turns the leak into a per-harness advantage. Placed above the
  scoring boxes, whose numbers it would bend.
- 2026-08-19 — added a box for checking the floor against what the server reports serving, above
  the scoring boxes rather than appended. The profile a run declares is a typed label: Hermes
  driven against a server that was not running scored `fail_test_failed`, which is the outcome
  a model that answered badly gets.
- 2026-08-19 — the sweep's wall clock is not usable as a headline number on this machine. At
  this context the runs swap, and the vision calls a swapped run void rather than slow. Every
  row carries its swap delta, so timings are separated at analysis time rather than averaged;
  the deciding metric is the server's own token counters and is unaffected.
- 2026-08-19 — the archived `opencode-ai/opencode` is not this project's predecessor, which the
  entry above assumed: its notice continues to Crush under the Charm team, while
  `anomalyco/opencode` is a separate lineage of the same name. The identifier scored here is
  unchanged; what falls is the idea that following the archived repository arrives at it.
- 2026-08-19 — the three open questions are answered and the section goes with them. Pi did not
  win cold, so the Hermes-warm tiebreak never arose; the switching bar was a token win costing
  no quality, and Pi's costs two tasks in fifteen; and the offline axis separates nothing, since
  the incumbent holds it too. The answers are in `docs/TECH.md`.
