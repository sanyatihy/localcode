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
- [ ] 2026-08-17 · 0002 · BLOCKING · What stack should the harness and scripts be written in? Leaning: Go — kit is Go, kit help go-checklist exists, and one static binary avoids a Python runtime competing with the model for the 32 GB the model needs. Counter-case: MLX (0006, 0009) is Python-only, so a Go harness means two languages if MLX wins.
