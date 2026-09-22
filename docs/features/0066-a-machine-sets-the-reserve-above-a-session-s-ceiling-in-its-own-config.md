---
id: 0066
title: A machine sets the reserve above a session's ceiling in its own config
status: Submitted
created: 2026-09-22
submitted: 2026-09-22
needs:
---

## Problem

The reserve above a session's ceiling is a quarter of the window, so it grows with the
window while a tool result does not: at 98,304 served it took 26,624 tokens where the same
clamp and the same measured overshoot need 16,384 at 49,152. Changing the constant would
have been one machine's setting in the repository, against 0064's rule.

## Non-goals

- Changing the derived share. It is the measured default and stays.
- Measuring the overshoot at larger windows. The sessions now running on the node produce
  those rows; sizing the cap from them is a later reading.

## Design

`NewLimits` takes a result cap in tokens, 0 for the derived share, and the launcher reads
`RESULT_CAP_TOKENS` from `~/.config/localcode/localcode.env`, a launcher settings file beside
`endpoint`, `ssh`, `serve.env` and `claude-code.env`, in the same `KEY="value"` form and
parsed by the harness's parser, now exported. The cap sets both the reserve and the byte
clamp the hook applies to a result, so the reserve stays a bound. A flag was not added: the
value belongs to a machine and not to a run.

## Tasks

- [x] RESULT_CAP_TOKENS in ~/.config/localcode/localcode.env caps what one tool result adds and so sizes the reserve, the derived share stays the default, covered by tests, and README says so

## Log

- 2026-09-22: began as a capped constant in `internal/chain/chain.go` and was redirected by
  the owner to a local setting before it was committed.
