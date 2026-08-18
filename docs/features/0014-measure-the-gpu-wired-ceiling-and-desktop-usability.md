---
id: 0014
title: Measure the GPU wired ceiling and desktop usability
status: Draft
created: 2026-08-18
shipped:
check:
checked:
review:
needs:
related: 0004, 0010
---

## Problem

At 64k context the machine glitches: windows stop rendering correctly and the editor
freezes. 0003's ladder called that cell **ok**, because its pass criterion was whether the
model completed a request — not whether the machine remained usable, which is the
vision's actual constraint. Every number it collected (resident size, free memory, swap
delta) is blind to the failure: nothing swapped, and nothing was ever going to.

The mechanism is GPU-wired memory. `iogpu.wired_limit_mb` is `0`, the system default of
roughly 75% of 32 GB, and the model holds 20.27 GB at 64k — mostly wired Metal buffers.
What is left has to carry WindowServer, the compositor and the editor.

## Non-goals

- **Not re-running 0003.** Its measurements stand; they answered the question they asked.
  This adds the question they did not.
- **No permanent system tuning without evidence.** Raising the wired limit is one of the
  options being measured, not the starting position, and a machine that needs undocumented
  `sysctl` state to be usable is a worse outcome than a smaller context.
- **No quality measurement.** Whether a config is *good* is 0004's and 0005's business.
  This decides which configs are *admissible* on a machine someone is using.

## Design

**The pass criterion is the desktop, not the model.** A cell passes only if the machine
stays usable while the model is under load, so the metric has to come from outside the
model process. Two candidates, both cheap:

- **Frame/compositor health** — `WindowServer` CPU, and whether a scripted UI interaction
  completes within a threshold.
- **Wired-memory headroom** — total wired against the limit, sampled during a full-context
  run. This is the number that should have been on the ladder and was not.

**Sweep the wired limit as a variable, not a fix.** `sudo sysctl iogpu.wired_limit_mb=N`
raises the ceiling and takes the memory from everything else, so it trades model headroom
against desktop headroom rather than creating any. Measure at the default and at one or
two raised values, and record the exact revert (`sudo sysctl iogpu.wired_limit_mb=0`).
This box is inherited from 0003, where it was removed as premature on the argument that
nothing approached a ceiling — which was wrong, because the ceiling being approached was
not one swap or RSS could show.

**Find the threshold, not just the verdict.** 32k has been used all session without
complaint and 64k breaks; the interesting answer is where between them the desktop starts
degrading, because that is the number that bounds the attended profile.

**This collides with Hermes.** Hermes refuses any context under 64,000 tokens, so if 64k
is not admissible while someone is using the machine, Hermes is not admissible for
attended work on this hardware — regardless of how it scores. 0010 has to state that as a
constraint rather than discover it as a bad result.

## Tasks

- [ ] Wired memory and its limit are sampled during a full-context run, and added to the ladder's row so the metric exists at all
- [ ] A desktop-usability check runs alongside a loaded model and produces a pass/fail that does not depend on the model's own success
- [ ] The context ladder is re-walked against that criterion at the default wired limit, and the degradation threshold between 32k and 64k is identified
- [ ] The effect of raising `iogpu.wired_limit_mb` is measured at one or two values, with the exact revert command recorded and the trade stated in both directions
- [ ] `docs/TECH.md` records the admissible context range for a machine in use, separately from the range the model can serve
- [ ] The consequence for Hermes' 64k floor is written down where 0010 will read it

## Open questions

- Is the right remedy a smaller context, a raised wired limit, or accepting that unattended
  runs get the machine to themselves? Leaning **the third for the grind profile and a
  smaller context for attended** — but this is exactly what the measurement is for, and
  the answer may differ per profile, which the vision already expects.
- Should the ladder refuse to run cells that are known to break the desktop? Leaning
  **no, but warn**: the unattended profile legitimately wants them, and a tool that hides
  a measurable state is worse than one that reports it.

## Log
- 2026-08-18 — raised from a user report, not the instrument: at 64k the desktop stops
  rendering and the editor freezes, while 0003 scored that cell `ok` because it measured
  whether the model finished. Nothing it recorded could have caught it — the failure does
  not touch swap, and RSS does not distinguish wired GPU memory from the rest.
- 2026-08-18 — the ladder now samples wired memory and its limit every two seconds *during*
  the fill, not just around it, and reports peak wired and minimum headroom per cell. The
  limit is recorded with provenance: `iogpu.wired_limit_mb` reads 0 here, so the 24 GB
  figure is `default-assumed` from the documented 75% of installed RAM, not a reading, and
  headroom inherits that uncertainty. Idle wired measures 2.58 GB, which puts the 64k cell
  at roughly 22.9 GB against that assumed 24 GB — consistent with the reported failure, and
  the first number in this repo that could have predicted it.
- 2026-08-18 — building the probe found the reason the metric was not merely missing but
  unreadable: `memprobe.sh` assumed a 4 KB page and this machine uses 16 KB, so `vm_stat`
  figures were understated 4×. Recorded as a gotcha in `docs/TECH.md` and as an erratum
  against the affected data file. No conclusion in 0003 reverses — free memory reads as
  pinned near zero either way — but wired memory read off the same counter would have been
  wrong by the same factor, in the direction that makes an inadmissible config look fine.
