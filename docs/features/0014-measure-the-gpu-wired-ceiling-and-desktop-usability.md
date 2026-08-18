---
id: 0014
title: Measure the GPU wired ceiling and desktop usability
status: Shipped
created: 2026-08-18
shipped: 2026-08-18
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

**WindowServer CPU is the one being used**, sampled every two seconds against the model
under load and recorded as its own column beside the model's own `outcome`. The rule, fixed
before any run so that it is a rule and not a preference, in fractions of one core:

| verdict | condition |
|---|---|
| `fail_saturated` | sustained ≥ 0.90 cores over any 30 s window — the compositor cannot keep up |
| `fail_stalled` | sustained ≤ 0.02 cores over any 30 s window — the compositor is not drawing |
| `pass` | neither, over a run of at least 30 s |
| `not_applicable` | the condition is unattended |
| `insufficient_samples` | the fill was shorter than the sustain window |

Attended baseline with no model loaded measures **0.17–0.47 cores** — the low end a nearly
static screen, the high end an editor actively rendering — so both bounds sit well clear of
normal operation. Each run records its own baseline in the apparatus row, since that spread
is wide enough that quoting a single measured figure would be the weaker claim. A brief spike is the compositor doing its
job; the sustain window is what separates that from the compositor losing.

**Two things this cannot do, stated now rather than discovered later.** Passive CPU cannot
distinguish "nothing to draw" from "stuck and not drawing", so the verdict is only meaningful
when someone is driving the machine and unattended cells report `not_applicable` rather than
a pass they did not earn — which is why the scripted UI interaction is still worth building.
And the *direction* of the signal at the failure is unverified: a starved compositor might
saturate or might stall, and which one 64k produces is not yet known. The full series is
recorded per cell for exactly that reason, so the first re-walk calibrates the threshold from
a known-bad cell instead of confirming a guess.

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

- [x] Wired memory and its limit are sampled during a full-context run, and added to the ladder's row so the metric exists at all
- [x] A desktop-usability check runs alongside a loaded model and produces a pass/fail that does not depend on the model's own success
- [x] The context ladder is re-walked against that criterion at the default wired limit, and the degradation threshold between 32k and 64k is identified
- [~] ~~The effect of raising `iogpu.wired_limit_mb` is measured at one or two values~~ — **dropped**, see the log entry of 2026-08-18. Diagnostic only; no decision here turns on the answer, and the lever passes to 0004 for the unattended profile
- [x] `docs/TECH.md` records the admissible context range for a machine in use, separately from the range the model can serve
- [x] The consequence for Hermes' 64k floor is written down where 0010 will read it

## Running the re-walk

The band in question is 32k to 64k, at 8k granularity. Nothing below 32k is re-walked:
those cells are not in doubt and each costs its full ingest. Roughly 50 minutes.

    CELLS="config/ladder-32k-q8_0.env config/ladder-40k-q8_0.env config/ladder-48k-q8_0.env config/ladder-56k-q8_0.env config/ladder-64k-q8_0.env" \
      CONDITION=attended-worked-in ./scripts/ladder.sh

Three conditions on the run, all of which void it if missed:

- **The desktop must be driven.** A verdict from passive CPU means nothing against a static
  screen, which reads as a stall. Do ordinary work while it runs; that is what `attended`
  denotes and the run is measuring.
- **Other processes must fit underneath the model.** The apparatus row records the footprint
  and the ladder prints it before the first cell. At 19.1 GB for the model at 32k and 20.3
  at 64k, an apparatus above ~9 GB puts the machine into swap, and per 0003 a run that swaps
  measures the pager rather than the desktop. A browser alone was measured at 5.8 GB.
- **The last cell is the known-bad one.** 64k is expected to degrade the desktop for the
  ~13 minutes it ingests. That is the calibration point, not a mishap — it is the cell that
  tells the threshold which direction the signal moves.

## Open questions

- Is the right remedy a smaller context, a raised wired limit, or accepting that unattended
  runs get the machine to themselves? Leaning **the third for the grind profile and a
  smaller context for attended** — but this is exactly what the measurement is for, and
  the answer may differ per profile, which the vision already expects.
- **Where is the wired limit actually set?** The run makes the assumed 24 GB doubtful: the
  desktop collapsed at 22.29 GB wired with 1.71 GB of assumed headroom left, having survived
  22.18 GB at 56k. Either the limit is near 22.3 GB rather than 24, or the mechanism is not
  the cap at all. The raise experiment is what separates those, and it is now the decisive
  one rather than a formality — if the cap is genuinely 24 GB, raising it will change
  nothing, and that null result is the finding.
- ~~Does the desktop die at load rather than under load?~~ **Answered: at load.** The trace
  was flat from its first sample and the operator confirms the freeze began as the cell
  started, not partway through. The admissible range is therefore a property of the served
  config, not of how full the context gets, and a cell can be judged from a load plus a
  minute of sampling instead of a full fill — a 13-minute verdict becomes a 90-second one.
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
- 2026-08-18 — `memprobe.sh` assumed a 4 KB page where this machine uses 16 KB, so every
  `vm_stat` figure recorded before today is understated 4×. Gotcha in `docs/TECH.md`,
  erratum against the affected data file; no conclusion in 0003 reverses.
- 2026-08-18 — the desktop check is WindowServer CPU, sampled from outside the model process
  and reported as `desktop_verdict` beside the model's `outcome`, because collapsing those
  two columns is the mistake that produced this feature. Cumulative CPU *time* is recorded
  and rates derived by the caller: `ps %cpu` is a decayed average over a window macOS does
  not document, and a threshold set against an undocumented decay is not reproducible.
  Thresholds are two-sided and provisional — the direction of the signal at the failure is
  the one thing the re-walk still has to establish.
- 2026-08-18 — the re-walk band is 32k/40k/48k/56k/64k, and the ladder takes a `CELLS`
  selection so a band can be walked without paying for cells that are not in question.
  Two harness bugs surfaced from one accidental full-ladder run and are fixed: the health
  check could conclude a server had died before `serve.sh` had `exec`ed it, recording
  `load_failed` for a server that then loaded fine and collided with the next cell; and
  `CELLS` used `:-`, so an explicitly empty value walked every cell instead of none. The
  first had hidden through every earlier run and appeared only under a 20 GB apparatus —
  the condition this feature exists to measure.
- 2026-08-18 — the band was walked attended at the default wired limit: 32k, 40k, 48k and
  56k pass; **64k fails**. Every cell returned `ok` from the model, so the ladder recorded
  five successes and one unusable machine — the split this feature was opened to expose.
  The attended ceiling is therefore **56k**, and it is a measurement rather than the
  extrapolation 0004 was gated on.
- 2026-08-18 — the direction of the signal is **stall, not saturation**, which answers the
  question the thresholds had to be two-sided about. WindowServer held 0.16-0.18 cores
  through 40k/48k/56k and sat at 0.02 for the whole of 64k; the failing cell's *peak* is
  below every passing cell's *minimum*, so the two populations do not overlap. The stall
  bound earned its place and the saturation bound has never fired.
- 2026-08-18 — **the wired-ceiling explanation did not survive its own measurement**, so
  the mechanism this doc's Problem states is unconfirmed: what is confirmed is the failure
  and where it starts. The `default-assumed` label on the 24 GB limit carries the whole
  disagreement, which is why it was recorded with provenance rather than as a reading.
- 2026-08-18 — the freeze is confirmed to begin at load, from two independent directions: the
  compositor trace is flat from its first sample, and the operator reports the desktop going
  unusable as the cell began rather than partway through its 13-minute fill. This is the
  cheapest finding here — verdicts no longer need a full fill, so the grid 0004 has to filter
  costs minutes rather than hours.
- 2026-08-18 — **the 56k ceiling is conditional on a cleared desk, and the condition matters
  more than the number** — the apparatus and working-set figures are in `docs/TECH.md`. The
  consequence for this repo's labels: `attended-worked-in` has been covering two very
  different machines, and this band measured the generous one.
- 2026-08-18 — **box 4 dropped and the feature shipped at five of six.** Raising
  `iogpu.wired_limit_mb` would separate "the cap binds lower than assumed" from "the cap is
  not what binds", and nothing here turns on which. The admissible range is measured either
  way; the remedy is a cleared desk either way; and Hermes is out for attended work either
  way. The lever passes to 0004, where it is a real question for the *unattended* profile —
  whether the cap is what stops a larger quant loading at all. The mechanism stays
  unresolved in the open questions rather than being quietly resolved by assumption, and
  `sudo sysctl iogpu.wired_limit_mb=0` remains the revert if anyone raises it later.
- 2026-08-18 — kept over the 150-line alarm deliberately: what is long here is a design whose
  thresholds had to be fixed before any run, a campaign that changed the plan eleven times,
  and a mechanism left open on purpose. The instrument's durable half — the verdict rule, the
  band, the conditions on the ceiling — is in `docs/TECH.md`, and what stays is why.
