---
id: 0025
title: Take the decode speedup the driver's own preamble makes admissible
status: Draft        # Draft | Shipped | Dropped — kit ship and kit drop write it
created: 2026-08-23
shipped:             # written by kit ship, never by hand
needs:               # feature ids that must ship first, e.g. 0002, 0003. Empty = can start now
---

<!-- `needs:` is the only field that changes what `kit next` offers, so it earns care.
     Leave it empty if this could be built today against what already exists — that is
     the common case, and several empty ones is what lets agents work in parallel.

     Judge it by whether the work could START now, not by what it touches. Clean
     boundaries hide couplings that make a feature dependent anyway:
       - a shared composition root or wiring function both features must edit
       - a shared router, registry, or dispatch table both add an entry to
       - numbered files in one sequence — migrations above all. Two features each
         adding "the next number" merge cleanly and break at runtime.
     Any of those means the second feature needs the first, however separate they look. -->


## Problem

Decode is 90% of the driver's wall clock: a real two-hour chain generated 3,450 tokens in
its first session alone at **4.5 tok/s**, and prefix caching had already reduced ingest to
the other 10%. The model's own MTP head is a measured **1.26×–1.40× on decode** at the
depths a session runs at, and it is refused at 49,152 and admissible at 32,768 — a limit
`config/agent.env` runs straight into. That context was chosen for the editor, whose first
request measured 36,309 tokens; the driver's preamble is **4,395**, so the reason the
driver cannot have the speedup is a number that was never about the driver.

## Non-goals

- **Not a change to `config/agent.env`.** It serves the editor flow, whose 36,309-token
  first request is what 49,152 was measured against. A second config is the answer if this
  one wins, and the editor keeps what it needs.
- **Not a re-measurement of MTP itself.** 0017 settled the ratio, the acceptance rate and
  the losslessness; this asks only whether the driver can afford the context that admits it.
- **Not a change to the reserve.** Measured on real work, a session overshot its ceiling by
  8,570 tokens against a 10,240 reserve, so the quarter-window it holds back is carrying
  what it was sized for.

## Design

**A driver config at 32,768, against `agent.env` at 49,152, on the same real instruction.**
Everything else is held: same fixture, same tool set, same sampling, same chat template.

**The trade is decode against handoffs, and it may not pay.** A smaller served context is a
smaller ceiling and therefore more sessions, and each extra session costs a preamble
re-ingest and a handoff generated at decode speed. The arithmetic, before any run:

| served | prompt budget | ceiling | room above a 4,395 preamble |
|---|---|---|---|
| 49,152 | 40,960 | 22,528 | 18,133 |
| 32,768 | 24,576 | 10,240 | 5,845 |

Three times the sessions, against a decode ratio of about 1.3. That is close enough that
the answer is not obvious from arithmetic, which is why this is a run and not a decision.

**Wall clock to a finished instruction is the measure, not tokens per second.** A chain that
is faster per token and slower to the answer has lost. The scorer is the fixture's own
tests, as 0023's was: what a session claims about itself is not evidence.

**The desktop rule applies before the suite.** 0014's ceiling governs, and MTP at 32,768 was
already screened against it in 0017.

## Tasks

- [ ] a driver serving config at 32,768 with the MTP head, refused loudly if the allocator
      or the desktop rule says no
- [ ] one instruction run to completion under both configs, scored by the repository's own
      tests rather than by what the sessions reported
- [ ] the two chains compared on wall clock to the finished instruction, sessions spent,
      tokens ingested and tokens generated
- [ ] `docs/TECH.md` records which context the driver serves and the number that decided it

## Open questions

- Whether the answer differs by task shape. A chain that reads widely pays the smaller
  ceiling hardest, and one that writes long files pays decode hardest; a single fixture
  cannot separate them, and a second would double the wall clock of the sweep.

## Log
