---
id: 0064
title: Keep a machine's serving settings in its home and the defaults in the repository
status: Draft
created: 2026-09-22
submitted:
needs:
---

## Problem

Raising what the node serves meant a commit, a branch and the node's checkout moved onto
it, and the setting lasted only as long as the checkout stayed there. A measurement then
depended on which branch the node happened to be on. The repository should hold defaults,
and what one machine runs should be that machine's state.

## Non-goals

- Overrides for the measurement scripts. `scripts/serve.sh` reads the config it is given
  and nothing else, so a row's config is the file it names.
- Choosing the runtime. 0063's state file, beside this one.

## Design

The daemon's wrapper, `scripts/node/serve.sh`, applies `~/.config/localcode/serve.env` over
`config/node.env` and serves the merged file it writes to
`~/.local/state/localcode/serving.env`, naming it in the banner. Sourcing the override into
the environment was rejected: `scripts/serve.sh` sources its config with `set -a` and would
overwrite it, and a merged file is one thing a reader can open to see what is served. The
override lives under `~/.config` beside `endpoint` and `ssh`, and the merged file beside the
switch, since both are the serving user's and the daemon sets `HOME` to that user's.

## Tasks

- [x] The daemon's wrapper applies ~/.config/localcode/serve.env over config/node.env, serves the merged file and names it, covered by a test, and README and TECH say so

## Log

- 2026-09-22: the spike began as a one-line context change committed to `config/node.env`
  and reverted: the owner ruled that the repository holds defaults only.
