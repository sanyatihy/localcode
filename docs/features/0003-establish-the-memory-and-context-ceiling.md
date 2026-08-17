---
id: 0003
title: Establish the memory and context ceiling
status: Draft
created: 2026-08-17
shipped:
check:
checked:
review:
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

Measure rather than model, because the arithmetic omits compute buffers, the graph, and
macOS's own reservation. For each context in a ladder — 16k, 32k, 48k, 64k — with KV at
f16, q8_0 and q4_0, load the baseline and drive the context genuinely full, then record
peak wired memory, peak resident, whether the GPU wired limit was hit, and whether the
system swapped.

**Filling the context is the point.** Allocation at load time understates the true peak,
and a config that loads and then dies at 30k is the exact failure this feature exists to
prevent.

Two ceilings get recorded, and **each belongs to one profile**:

- **Hard ceiling** — the largest config that runs at all, with the machine to itself.
  This is the *unattended* budget: nobody is using the desktop, so the model may have it.
- **Working ceiling** — the largest that leaves the machine usable with an editor and a
  browser open. This is the *attended* budget, because attended use means those are
  running by definition.

Binding them to profiles is the point rather than a labelling nicety: a config
recommended for attended work on numbers measured with nothing else running is a
recommendation that will swap the first time it is used for real.

`iogpu.wired_limit_mb` is measured at its default first. Raising it is tested as a
separate, explicitly reversible step, and reported as a distinct result — an option with
a cost, not the new baseline.

Swap is a **failure**, not a slow pass. A config that swaps has left the envelope
regardless of what it scores — and every timing taken while paging measures the pager,
so such a run is void rather than merely poor.

**This is already happening at the baseline, which is why the feature is not theoretical.**
Measured 2026-08-17 with the 32k/q8_0 baseline loaded, the server *idle*, and ordinary
desktop apps open: 0.02 GB free, 13.9 GB of 14.3 GB swap in use, 1.32 GB compressed, and
swap still growing ~74 MB per 8 seconds at idle. `llama-server` alone was 18.45 GB of
32 GB. So the attended working ceiling at 32k appears to be **exceeded before the context
is filled**, and the wildly inconsistent prompt-processing rates observed during 0002's
work — 4.5 to 114 tok/s for comparable operations — are the expected symptom.

**Measured anchor, from the server already running.** Qwen3.8-27B Q4_K_M at 32k context
with `q8_0` K and V, all layers on GPU, reports ~19.3 GB resident. That is a starting
point and explicitly **not** a ceiling: it was observed at low context occupancy, and KV is
not fully allocated until it is used. It is recorded here because the ladder should start
from a real number, and because the gap between it and this feature's filled-context
measurement is the whole reason the feature exists.

Cold prompt processing on that same server measured **91.8 tok/s** over 7,024 uncached
tokens. At that rate a full 32k ingest is roughly six minutes, so **prompt-cache reuse is
not an optimisation here but the thing that makes the context usable at all** — which makes
cache-invalidating behaviour a first-class risk for 0010 to score.

## Tasks

- [ ] A script drives a served config to genuinely full context and records peak wired memory, peak resident, and swap activity
- [ ] The ladder (16k/32k/48k/64k × f16/q8_0/q4_0 KV) runs unattended and writes one row per cell, marking each pass, swap, or OOM
- [ ] Hard ceiling and working ceiling are both identified and each named with the profile it bounds — hard for unattended with the machine to itself, working for attended with an editor and browser open
- [ ] The baseline 32k/q8_0 config is re-measured under both conditions, since it is already observed swapping at idle with apps open, and the result says plainly whether it is viable for attended use at all
- [ ] The effect of raising `iogpu.wired_limit_mb` is measured separately, with the exact revert command recorded
- [ ] `docs/TECH.md` states the measured envelope, what happens past it, and the ladder for 0007 to reuse

## Open questions

- Is q4_0 KV worth carrying forward? It halves KV again, but long-context retrieval is
  precisely what agentic coding depends on. Leaning: **measure it here, let 0004 judge
  the quality cost** — cheap to include now, expensive to re-run later.

## Log

## Log

- 2026-08-17 — the two ceilings are now bound to the two profiles rather than being
  presented as a pair of numbers: hard is the unattended budget, working is the attended
  one. Prompted by measuring the baseline while ordinary apps were open and finding the
  machine already paging at idle, which means the numbers 0002 collected during its build
  were taken outside the envelope this feature exists to establish.
- 2026-08-17 — scope correction. This feature measured *one* envelope: Qwen3.8-27B at
  Q4_K_M, contexts to 64k, on 32 GB. Memory not binding there does not rule memory out —
  Q6_K weights are roughly 6 GB heavier, and larger models and longer contexts are
  untested. 0004's quant sweep is where memory gets its next real chance to bind, and the
  planned 128 GB machine moves every rung at once, which makes the hardcoded ladder a
  defect rather than a setting.
