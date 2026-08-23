---
id: 0003
title: Establish the memory and context ceiling
status: Shipped
created: 2026-08-17
shipped: 2026-08-17
needs: 0001
related: 0004
---

## Problem

On 32 GB, context is the scarce resource and agentic coding is what spends it: every
turn re-reads files and tool output. Q4_K_M weights are ~16.4 GB and KV costs ~256
KiB/token at f16, so the arithmetic says 32k context lands near 22 GB and 64k does not
fit — but that is arithmetic, not measurement, and 0004 cannot sweep a range whose top
is a guess. Guessing high means discovering the ceiling as an OOM or a swap storm
mid-sweep.

## Non-goals

- **No quality judgement.** Which context is *best* is 0004. This finds what is *possible*.
- **No model comparison.** Other models' envelopes belong to 0007, using this method.
- **No permanent system modification.** Any `sysctl` change must be reversible and
  recorded; a machine that needs undocumented tuning to boot into a working state is a
  worse outcome than a smaller context.

## Design

Measure rather than model, because the arithmetic omits compute buffers, the graph and
macOS's own reservation. For each context in a ladder — 16k to 64k — with KV at f16, q8_0
and q4_0, load the baseline and drive the context genuinely full, then record peak wired and
resident memory, whether the GPU wired limit was hit, and whether the system swapped.

**Filling the context is the point**: allocation at load time understates the peak, and a
config that loads and dies at 30k is the failure this exists to prevent.

Two ceilings, each bound to one profile — **hard** is the largest config that runs with the
machine to itself (unattended), **working** the largest that leaves the desktop usable
(attended). Binding them to profiles is the point: a config recommended for attended work on
numbers measured with nothing else running will swap the first time it is used for real.

**Swap is a failure, not a slow pass**, and every timing taken while paging measures the
pager, so such a run is void rather than poor. `iogpu.wired_limit_mb` is measured at its
default first; raising it is a separate reversible step reported as its own result.

Two things the first run established about the instrument itself. **Peak RSS is not usable
under saturation** — free memory sat at 0.01–0.02 GB in every cell, so resident size is
clamped by what physically fits: it moved 18.10 → 18.92 GB across a fourfold context range
where the KV arithmetic predicts ~3.4 GB more. And **swap delta is confounded by the
ladder's own restarts**, since tearing down an 18 GB process releases more pressure than the
next cell creates. Fill duration and prompt rate are the metrics that discriminate.

Cold prompt processing measured **91.8 tok/s** over 7,024 uncached tokens, so a full 32k
ingest is roughly six minutes: **prompt-cache reuse is what makes the context usable at
all**, which makes cache-invalidating behaviour a first-class risk for 0010 to score.

## Tasks

- [x] A script drives a served config to genuinely full context and records peak wired memory, peak resident, and swap activity
- [x] The ladder writes one row per cell recording **time to ingest a full context** and the server's prompt rate, not only resident size — under saturation RSS stops discriminating exactly when the answer matters
- [x] The ladder runs in both conditions, labelled, and the same cell is compared across them rather than across configs within one
- [x] The ladder is extended upward — 48k and 64k — because no cell from 8k to 32k failed in either condition, so the ceiling is above everything measured so far and remains unbounded
- [x] Hard ceiling and working ceiling are both identified and each named with the profile it bounds — **answered by refutation: neither is reached at any context this model supports in practice.** 64k/q8_0 runs at 20.27 GB with zero swap, and marginal cost above 16k is a steady 31-37 KB/token, so exhausting the remaining headroom would take hundreds of thousands more tokens. Memory does not bound context on this machine
- [x] `docs/TECH.md` records the full ladder, the ingest-time curve, and the exact envelope the result is scoped to — one model, one quant, contexts to 64k, 32 GB — so it is not read as a general claim about memory, plus the gotchas each wrong number here already cost
- [x] Ladder rungs are derived from available memory rather than hardcoded, since a 128 GB machine is planned and every rung here is a fact about a 32 GB one
- [x] The baseline 32k/q8_0 config is re-measured under both conditions, since it is already observed swapping at idle with apps open, and the result says plainly whether it is viable for attended use at all

## Log

- 2026-08-17 — the two ceilings are bound to the two profiles rather than presented as a pair
  of numbers: hard is the unattended budget, working the attended one.
- 2026-08-17 — the first attended run is kept, relabelled `attended-worked-in` with
  `instrument: pre-timing`. A reboot destroys the state it captured — hours of uptime with
  swap already allocated — so it is the only reading of a genuinely worked-in machine until
  the next long session. Three conditions, not two.
- 2026-08-17 — **the premise is refuted, which is the most useful thing this could produce.**
  The full q8_0 ladder — 8k to 64k — passed with swap delta 0.0 MB at every rung. Memory is
  not the binding constraint in this envelope; ingest time is, and a cold 64k context costs
  13.1 minutes against 5.4 at 32k. The measurements are in `docs/TECH.md`.
- 2026-08-17 — peak RSS came out *higher* unattended than worked-in for four of five cells,
  which supports RSS being clamped by eviction under pressure rather than reflecting demand.
- 2026-08-17 — consequence for 0004: 64k/q8_0 sits at 20.27 GB of 32 GB, so **larger quants
  are worth sweeping**. Q5_K_M and Q6_K were excluded on a memory argument the measurement
  does not support.
- 2026-08-17 — scope correction: this measured *one* envelope — Qwen3.8-27B at Q4_K_M, to
  64k, on 32 GB. Memory not binding there does not rule memory out, and the planned 128 GB
  machine moves every rung at once, which makes a hardcoded ladder a defect rather than a
  setting.
- 2026-08-17 — the `iogpu.wired_limit_mb` box is removed rather than done: it existed to
  answer "can the ceiling be raised", and nothing from 8k to 64k came near one. The question
  belongs to 0004, where Q5_K_M and Q6_K are the first configs with a real chance of meeting
  a wall. It also needs `sudo`, which is a human's to grant.
- 2026-08-17 — `scripts/rungs.sh` derives the rungs from the machine and names which
  constraint binds, replacing the hand-written ladder. It reproduces the same four rungs,
  and modelling more memory does not move them — the numbers are in `docs/TECH.md`.
- 2026-08-17 — open question settled: at 32k, q4_0 KV used 18.66 GB against q8_0's 19.13 and
  filled in 325 s against 326 — 0.47 GB and no time at all. Since memory does not bind here,
  **q4_0 buys nothing** and q8_0 is the better default. Worth revisiting only if a larger
  quant makes memory bind.
