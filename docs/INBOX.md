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
- [ ] 2026-08-19 · 0011 · FYI · kit needs a rule against comments that duplicate docs — take it upstream. Evidence here: comments are 23% of this repo's Go (685 of 2,942 lines), and the densest files are the newest, because each agent matches the last one's density. My own hermes.go is 40% and its two largest blocks restate harness/README.md and docs/TECH.md verbatim in substance — a fact with two homes, which kit already forbids for docs and does not check for code. Leaning: audit the duplication, not the ratio — shingle-match comment text against docs/**.md and flag the overlap, since that finds the actual defect and cannot be gamed by writing longer functions. A per-package ratio is worth having only as a smoke alarm behind it, because a budget rewards padding functions and deleting the cheap comments that earn their place: a trap, a measured number, or a refusal.
