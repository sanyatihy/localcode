---
id: 0042
title: Settle whether the GPU wired cap is what binds
status: Draft
created: 2026-08-25
shipped:
needs: 0041
---

## Problem

The largest speed lever this project has is refused by an allocator failure nobody has
attributed: the model's own draft head is worth 1.26–1.57x on decode, decode is 73–81% of a
chain's clock, and at 49,152 the first prefill batch fails
`kIOGPUCommandBufferCallbackErrorOutOfMemory`. 0041 screened the one software cause anyone
had named — the 8 GiB host prompt cache — and cleared it, so what remains is the machine's
own GPU wired cap, which is asserted to bind and has never been moved.

## Non-goals

- No re-walk of the context ladder. Nine cells is its own feature and it is worth nothing
  until this says the cap is what bound them.
- No change to the driver's default context. 0034 settled 32,768 on a whole-chain result
  rather than on the draft head, and a head admitted at 49,152 does not revisit that.
- No permanent machine change. `iogpu.wired_limit_mb` is a sysctl and a reboot restores it;
  anything needing a boot argument is out of scope and stays out.
- No second instrument. `scripts/screen.sh` already decides admissibility for the price of a
  load, and 0017 is the precedent for reading two of its rows against each other.

## Design

**The quantity that fails is not the quantity this repo measures, and that is why an
experiment is the only route.** `vm_stat`'s wired count is the whole machine's; the cap
governs the GPU's share of it. Metal publishes a device's `currentAllocatedSize`, but per
process — a probe reads its own allocations and never llama-server's — so what crosses the
cap is not observable from outside the server at all. Intervention is what is left: move the
cap, screen the same config, and read the verdict.

**The evidence says whole-machine wired is not the discriminator, so the negative headroom
this repo now reports is not itself the finding.** 0014's desktop band served 49,152 at a
22.21 GB peak and 65,536 at 22.29 GB with every request succeeding, both above the 21.33 GiB
cap. The draft head refuses at 49,152 at 22.39 GB and serves a 35,020-token prompt at 38,912
at 22.28 GB. A non-speculative config therefore works at a wired total a speculative one
fails just past, which is what rules the total out as the thing that binds and leaves the
draft context's own allocation as the candidate.

**One rung, and the rung is not a new number.** 24,576 MiB is what every `wired_headroom_gb`
in this repository before 2026-08-25 was computed against, so raising to it makes the
arithmetic those rows were reported under retroactively true rather than inventing a
ceiling. It clears the observed 22.39 GB peak by 2.2 GB and leaves 8 GiB to the system. If
the head clears there, the walk stops there.

**A second candidate is refused by the same cap and costs one more load.** 0017 screened
MTPLX — the MLX runtime with native MTP — and recorded `loads, then out of memory under a
real prompt`. Its shipped `Optimized-Speed` checkpoint is documented to peak at 23.6 GB,
which is 22,504 MiB against the 21,845 MiB Metal derives here: **659 MiB short**, and the
rung above clears it by 2.1 GB. Memory was the only thing standing between it and a verdict,
and that verdict is worth more than a second reading of the cap — per 0017's own non-goals
MTPLX serves the Anthropic `/v1/messages` 0008 needs and reports acceptance and cache state,
which are two of the three reasons 0006 stayed on llama.cpp. Screening it here does not
reopen the runtime question; it removes the reason that question could not be asked.

**A raise is measured unattended and adopted only attended.** VISION's constraint is that no
config may make the machine unusable for the editor and browser the developer runs while the
agent works, and that constraint bites harder at a raised cap than anywhere else in this
project: the failure mode of starving the system side of unified memory is a freeze, not an
error. So the screen runs with nobody at the keyboard, and 0014's desktop verdict — the half
that needs a desktop to lose — is what decides adoption rather than the load.

**What must hold afterwards is an attribution, not a speedup.** Either the cap is what
refused the head, and the row that says so was taken at a named cap; or it is not, and the
draft head at 49,152 is closed for a reason this project has finally established.

## Tasks

- [x] a raise is applied and undone by one command, and `scripts/memprobe.sh` reports the
      value in force so every row taken under it says which cap it was taken at
- [x] `config/mtp-49k.env` screens admissible or not at 24,576 MiB, unattended, against the
      same 44,236-token filled prompt 0017 and 0041 used
- [ ] MTPLX screens admissible or not at the same 24,576 MiB, unattended, against a prompt
      sized to the context it is served, and its row records which checkpoint
- [x] the desktop verdict at the raised cap is taken attended with the head serving, or the
      box is dropped because the `config/mtp-49k.env` screen said the cap is not what binds
- [ ] `docs/TECH.md` says what refused the draft head at 49,152, and `config/machine.json`
      carries the cap this machine is held to or is explicitly left alone
- [ ] `docs/TECH.md`'s "MTPLX has no route left" is corrected or confirmed, and
      `docs/VISION.md`'s listing of it among the permanent exclusions with it

## Open questions

- **Whether a cleared head changes the two profile ceilings.** 57,344 and 65,536 were walked
  against the desktop at the derived cap, and the failing cell stalled the compositor from
  its first sample. Raising the cap moves what the GPU may take from the system side, which
  is the side the compositor is on, so the ceilings could move either way. Leaning towards
  measuring nothing here and letting the ladder re-walk be its own feature.
- **Whether MTPLX belongs in this feature at all.** This doc's problem is an attribution —
  what refused the draft head — and MTPLX is 0006's runtime question riding on the same
  intervention. Against keeping it: two questions in one doc is how a result stops being
  readable. For keeping it: the experiment is identical, the marginal cost is one load, and
  a verdict withheld for memory is not a runtime comparison until it exists. Leaning towards
  keeping it, and splitting only if the screen makes the runtime question live.
- **Whether `wired_headroom_gb` should be reported at all.** It is a whole-machine figure
  against a GPU-share cap, and this feature rests on those being different quantities. It has
  now been read as a headroom three times. Leaning towards renaming rather than removing, once
  this says what the cap governs.

## Log

- 2026-08-26 — **MTPLX added as a second candidate on the same raise, and the docs that
  ruled it out added to the last box.** TECH.md contradicts itself in one paragraph: it
  grants `mlx_lm` the raised cap as "the 'more memory' this paragraph ruled unavailable" and
  then says "MTPLX has no route left", and VISION inherits that as a permanent exclusion.
  MTPLX is 659 MiB short of the derived cap, so the route is the one this feature already
  walks. Scope tension recorded as an open question rather than settled here.

- 2026-08-27 — **the attended verdict cost three runs, and two of them failed the instrument
  rather than the machine.** The first swapped 601.9 MB with apps at 11.7 GiB and voided; the
  second came in clean on swap at 6.9 GiB but scored `fail_stalled` on a desktop nobody was
  driving. Only the third satisfied both at once. The rule reads sustained WindowServer load
  over a 30 s window, so an operator who stops interacting scores identically to a compositor
  that has died — the second run reported `fail_stalled` at a *lower* wired peak than the run
  it passed, which is backwards if the cap is the cause. Recorded here because it is evidence
  for the INBOX question 0017 already opened about whether this rule can see the state it is
  for, and because the next person to take an attended verdict needs to know it is an
  eleven-minute commitment rather than a command.
