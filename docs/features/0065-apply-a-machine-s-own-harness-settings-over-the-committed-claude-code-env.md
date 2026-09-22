---
id: 0065
title: Apply a machine's own harness settings over the committed claude-code.env
status: Submitted
created: 2026-09-22
submitted: 2026-09-22
needs:
---

## Problem

A session on the node retried one `Write` five times at five minutes each: the file's JSON
ran past the 8,192-token output cap, the truncated call could not be parsed, and the model
sent it again. The cap is a harness setting in the committed file, so raising it on the
machine that can afford it meant a commit, against 0064's rule that the repository holds
defaults and what one machine runs is its state.

## Non-goals

- Raising the committed default. The laptop's 9 tok/s makes a 16,384-token reply a half
  hour, and 0059 chose 8,192 for it.
- A launcher guard on a `Write` that fails to parse twice for the same reason. BACKLOG.

## Design

`EnvFromFile` appends the machine's `~/.config/localcode/claude-code.env` after the committed
file's variables, so the last entry of a name wins, which is what `declared` and the harness
already read. Beside `endpoint`, `ssh` and 0064's `serve.env`. A missing file changes
nothing; a malformed one is an error, as the committed file's would be.

## Tasks

- [x] EnvFromFile applies ~/.config/localcode/claude-code.env over the committed file, last entry winning, covered by a test, and README and TECH say so

## Log

- 2026-09-22: none.
