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
- [ ] 2026-08-19 · 0011 · FYI · How much repo context may a frontier drafter be shown? Leaning: whole-repo read for planning (docs, code, git history) but no secrets or data files, since planning quality depends on seeing the couplings — and the vision already names planning context as the permitted exception.
