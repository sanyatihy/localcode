---
id: 0024
title: Settle the prefill batch against ingest time
status: Draft        # Draft | Shipped | Dropped — kit ship and kit drop write it
created: 2026-08-22
shipped:             # written by kit ship, never by hand
needs:               # feature ids that must ship first, e.g. 0002, 0003. Empty = can start now
---

## Problem

Prefill is where a turn's time goes, and the flag that sizes it has never been set. The
editor turn in `docs/TECH.md` spent **434 s ingesting 36,309 tokens against 14.8 s
generating 82** — 97% of the clock on the half of the work `--ubatch-size` governs.
`scripts/serve.sh` passes neither batch flag, so every number this repo holds was measured
at llama.cpp's defaults with nobody choosing them.

## Non-goals

- **No decode work.** A batch size cannot move decode, whose ceiling is memory bandwidth;
  0017 already spent the one lever that beats it.
- **No context change.** Prefill costs what the prompt is rather than what the context
  reserves, which `config/agent.env` already records.
- **No second runtime.** `PREFILL_STEP_SIZE` is this axis on MLX, and re-opening that
  runtime needs the reversal condition 0006 recorded rather than a batch sweep.
- **No quantisation change.** Fewer bytes a token is the only thing that raises the decode
  ceiling, and it is 0004's axis.
- **Not a commitment to change the config.** The output is a decision with numbers behind
  it, and "the default was right" is one of its outcomes.

## Design

The two flags are `--batch-size` (logical, default 2048) and `--ubatch-size` (physical,
default 512). **Only `--ubatch-size` is swept.** It sizes the compute buffer and the Metal
dispatch; `--batch-size` is a ceiling on it and moves only where it would otherwise clamp
the value under test.

Both become config variables rather than flags at the call site. A variant is a new file,
which is the rule every other measurement here was produced under — a flag passed at the
call site leaves a recorded number attributable to no config.

**The metric is cold ingest at depth, not prompt tok/s on a short prompt.**
`scripts/ladder.sh` already reports cold ingest per rung and that is the column that has to
move. A batch size that wins at 512 tokens and loses at 32,000 has lost the case that
matters.

**The admissible range is found by refusal, not written down.** The sweep walks powers of
two up from the default until the allocator refuses or 0014's desktop rule fails, and
records where it stopped. Rungs written by hand are facts about one laptop, which is the
defect `scripts/rungs.sh` exists to avoid.

**Wired memory is the cost, and it is measured rather than assumed.** The compute buffer
grows with the batch, and 0017's adopted config already peaks at 22.10 GB where 0014's
desktop died at 22.29. Each cell is therefore screened against the desktop rule before it is
scored — the order 0017 used, and the reason two of its three candidates never ran a suite.

**Both profiles are swept, because they may not agree.** `config/tuned.env` serves 32,768
and `config/agent.env` 49,152, so the editor profile reaches any given batch size with less
headroom. 0017 adopted a mechanism for one profile and refused it for the other, so a split
outcome is precedent rather than a complication.

**A batch size that changes the answer is not a speedup.** Fixed prompts at temperature zero
are hashed across the admissible cells before the depth sweep runs: the batch size changes
how a prefill is split, floating-point reductions are not order-independent, and a
divergence would mean every number recorded at 512 describes a different model than the one
under test. The check is cheap and it gates the expensive one.

## Tasks

- [x] `scripts/serve.sh` passes `--batch-size` and `--ubatch-size` when a config sets them, and a config that sets neither produces the process line it produces today
- [ ] the admissible `--ubatch-size` range is found by walking up from the default until the allocator or 0014's desktop rule refuses, with wired peak and desktop verdict recorded per cell
- [ ] fixed prompts at temperature zero hash identically across the admissible cells, or the divergence is recorded and the sweep stops there
- [ ] cold ingest at depth is scored for every admissible cell at both profiles' contexts, into `docs/data/`
- [ ] a decision is recorded in `docs/TECH.md` with the numbers, the wired cost of the chosen value and what would reverse it, and the configs carry the winner or keep the default and say why

## Open questions

- If the two profiles want different values, do they get different values? Leaning **yes** —
  they already differ in context, and 0017 adopted a mechanism for the grind profile while
  refusing it for the editor, so per-profile is a shape this repo already ships.
- The desktop screen needs somebody at the machine, and every screen 0017 ran was
  `unattended`. Leaning: screen this sweep the same way and record the attended verdict as
  untaken, since that is the gap `BACKLOG.md` already names against the MTP config rather
  than a new one.

## Log
