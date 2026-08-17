---
id: 0008
title: Wire the winning config into the coding agent
status: Draft
created: 2026-08-17
shipped:
check:
checked:
review: human
needs: 0004, 0010
related: 0002
---

## Problem

The winning config has to survive a real editor session, not just the fixed suite. Cursor
is the specific target, and it carries a constraint that changes what "local" means here —
one that should be measured and stated rather than discovered halfway through a workday.

## Non-goals

- **No further config tuning.** If live use contradicts the scorer, that is a finding worth
  a new feature and evidence the suite is unrepresentative, not a quiet re-tune.
- **No CLI harness comparison.** 0010 settles Pi against Hermes; this is the editor.
- **Not making Cursor offline.** It cannot be, per the design below. Pretending otherwise
  by burying the tunnel in a setup script is the failure mode this feature exists to avoid.

## Design

**Cursor cannot reach a loopback endpoint.** It routes model requests through its own
backend rather than calling the base URL from the machine, and it rejects plain HTTP. A
local model therefore requires a public HTTPS tunnel, and the request path becomes:
Cursor client → Cursor's backend → tunnel → this Mac.

Three consequences, all of which belong in the result rather than in a footnote:

1. **The on-device property is gone in this mode.** Prompts and file context leave the
   machine even though inference is local. The vision permits this only as a named,
   bounded exception — so the exposure is what this feature measures, alongside the
   performance.
2. **Latency gains a round trip** through Cursor's backend and the tunnel, on every turn.
   Compared against the same config under a local CLI harness, that is the real cost, and
   0010's numbers are the baseline it is measured against.
3. **The tunnel is an open ingress** to a server on this machine while it runs. It must be
   authenticated and torn down with the session, never left running.

So the feature's honest output may be **"Cursor is not the right front-end for this
project"** — a supported conclusion, recorded with its numbers. `review: human` because
accepting the privacy trade is not an agent's call to make.

A verified local-CLI comparison runs in the same sitting: the same task through Pi or
Hermes with the network disabled, proving the on-device path still works and giving the
Cursor numbers something to mean.

## Tasks

- [ ] The tunnel is set up authenticated, HTTPS, and torn down with the session, with the exact commands committed
- [ ] Cursor completes a real task in this repo against the local endpoint, and the transcript is recorded
- [ ] What Cursor's backend receives is characterised — prompts, file context, repo metadata — and written down plainly
- [ ] Per-turn latency is measured against the same task run through the 0010 winner locally
- [ ] Context exhaustion is made visible rather than silently truncating history
- [ ] The same task is completed through the local CLI harness with the network disabled, proving the offline path
- [ ] A recommendation is recorded in `docs/TECH.md`, including "do not use Cursor for this" if that is what the numbers say

## Open questions

- Is the privacy cost acceptable given inference stays local? A human call, hence
  `review: human`. Leaning: **acceptable for exploratory editing, not for the unattended
  grind**, which is what 0011 would send to local agents anyway.

## Log

- 2026-08-17 — retargeted from "some OpenAI-compatible agent" to Cursor specifically, and
  the loopback limitation moved into the design where it changes the tasks.
