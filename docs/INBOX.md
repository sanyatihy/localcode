# Inbox

Questions an agent cannot answer alone. Agents append, a human drains.
Delete a line once it is resolved — git keeps the history.

Format, one per line:

```
- [ ] YYYY-MM-DD · <feature-id> · BLOCKING|FYI · <question, with your leaning>
```

State a leaning. It turns most questions into a yes/no.

`BLOCKING` stops `kit next` offering that feature until you answer. `FYI` is
recorded for you to see but does not block — use it for anything work can
continue without.

The marker decides how hard `kit audit` pushes, too. An unanswered `BLOCKING`
question is HIGH after two days, which fails `kit audit --strict` and so stops
anything shipping. An undrained `FYI` is LOW after a fortnight: visible, never
a build failure, because it was cleared to wait.
- [ ] 2026-08-19 · 0011 · FYI · kit needs a rule against comments that duplicate docs — take it upstream. The defect is a fact with two homes, which kit already forbids for docs and does not check for code: the worst offender here restated harness/README.md and docs/TECH.md verbatim in substance. Since cut by hand — this repo went 20.3% to 18.9% comment lines, and its densest file 35% to 22% — which is why the ratio is the wrong instrument to propose: it was never the thing that was wrong. Leaning: audit the duplication, not the ratio — shingle-match comment text against docs/**.md and flag the overlap, since that finds the actual defect and cannot be gamed by writing longer functions. A per-package ratio is worth having only as a smoke alarm behind it, because a budget rewards padding functions and deleting the cheap comments that earn their place: a trap, a measured number, or a refusal.
- [ ] 2026-08-20 · 0017 · FYI · 0014's desktop verdict passed a cell the operator reported as laggy, so the attended half of this feature's screen may be reading a rule that cannot see the state it is for. Measured: with the DFlash2 config resident at 22.16 GB wired, WindowServer sustained 0.37-0.50 cores against its own 0.31 pre-load baseline, and the rule fires only below 0.02 or above 0.90 — the failing 64k cell 0014 calibrated on stalled dead at 0.02, which is a different regime from busy-but-behind. Leaning: record each row's rise over its own pre-load baseline now and let a second operator report calibrate a bound, rather than inventing a threshold from one report; 0014's two existing bounds stay as they are either way.
