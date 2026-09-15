---
id: 0058
title: Re-screen on the node every candidate the laptop refused for memory
status: Draft
created: 2026-09-14
submitted:
needs: 0056
---

## Problem

Four candidates lost on the laptop for memory and for nothing else: the native MTP head is
refused at 49,152 by the GPU cap, the DFlash2 drafter fails at load, MTPLX runs out of
memory under a real prompt, and MLX stalls under pressure. VISION says such an exclusion
lifts only when measured on a machine with room, and the node has 36 GB and a 30,720 MiB
cap.

## Non-goals

- Adopting a runtime for the editor flow that serves no `/v1/messages`. MLX is measured
  for its numbers; the flow it cannot serve is 0006's finding and stands.
- The GB10. 0054 has its own runtime question.
- Fine-tuning, and any model other than Qwen3.8-27B.
- A larger quant. Q5_K_M and Q6_K stay excluded by projection: the owner does not want
  them tested, and the envelope stays Q4_K_M.

## Design

Every candidate runs on the node with nobody logged in and the daemon's server stopped,
against `config/node.env` as the baseline, at the window 0056's ladder settled. One
machine state, one toggle at a time.

Each candidate is screened first with `scripts/screen.sh` for the price of a load: peak
wired, headroom, and whether it generates one token. A candidate that swaps or cannot
generate is recorded as inadmissible with its numbers and goes no further. An admissible
candidate then runs `scripts/pair.sh` against the baseline on the ranking suite for the
decode ratio, the tier-1 suite at N=3 for pass rate and tool-call validity, and for a
speculative candidate the fidelity hash that proves it lossless.

The candidates and what each needs on the node: the native MTP head at the node's window,
`SPEC_DRAFT_N_MAX` swept as 0025 did; the DFlash2 drafter, which needs the scratch build
of llama.cpp PR #27342 named in `config/dflash2-49k.env`, built on the node by a script
this feature adds under `runtimes/`; MTPLX and MLX, each from its own committed venv via
`runtimes/*/setup.sh`, MLX with the cache bounds 0006 found mandatory.

TECH gets one table: candidate, admissible, peak wired, decode ratio, tier-1 pass, and
the reason where it stops. The node's served config changes only if a candidate beats
the baseline on decode without losing on pass rate, and that change is its own box.

## Tasks

- [x] The native MTP head is screened and paired at the node's window with the draft depth swept, and TECH records its ratio and fidelity beside the laptop's 32k figures
- [x] The DFlash2 build is produced on the node by a committed script and the drafter is screened; if admissible, paired and scored
- [x] MTPLX is set up from its venv on the node and screened; if admissible, paired and scored
- [ ] MLX is set up from its venv on the node with bounded caches and scored through `runtimes/mlx/compare.sh` against the baseline
- [ ] TECH carries the one table, and `config/node.env` changes only if a candidate beats the baseline on decode without losing pass rate, with the row that decided it cited

## Log

- 2026-09-15: the drafter is named by path rather than by `-hf` tag. The plan assumed
  `config/dflash2-49k.env` as written; that repository's main moved after 0017 and the tag
  now resolves to a 1.1 GB download this node's link cannot deliver. `scripts/serve.sh`
  gains `SPEC_DRAFT_MODEL` and `config/dflash2-node-49k.env` names the cached revision, so
  what served is recorded rather than resolved at run time.
- 2026-09-15: `scripts/pair.sh` takes the candidate's serve command and port. The plan has
  every admissible candidate paired by that script, and two of the four are not llama.cpp;
  `scripts/screen.sh` already took the serve command as arguments for the same reason.
- 2026-09-14 — paused: the remaining candidates wait for the owner's word
