---
id: 0068
title: Code on the node itself, with the model served for the session and the desk usable
status: Draft
created: 2026-09-25
submitted:
needs: 0067
---

## Problem

The node is to become the owner's only machine. The numbers it runs on were taken with
nobody logged in: a 6 GB reserve, a 10 GB headroom floor and a 163,840-token ceiling. The
serving daemon also keeps roughly 24 GB wired from boot onward. Once an editor and a browser
share the 36 GB, VISION's desk rule applies to this machine, and nothing yet says which
context it can carry in attended use.

## Non-goals

- Deleting the link daemon or the remote-endpoint path. The GB10 (0054) and any future node
  use them. The workstation role simply turns the link off.
- Retiring the laptop's measurements. They remain the record of the M2 Max envelope.
- Changing `config/node.env`. If the attended ceiling is below its context, the lower value
  goes in the workstation's own `~/.config/localcode/serve.env`, following 0064.

## Design

The model is served on demand. In the workstation role `SERVE_DAEMON` is off, and the
launcher starts and stops the server for each session, as it did on the laptop. This frees
the memory whenever the owner is not coding. The launcher serves `config/node.env` with
`serve.env` applied. The code that merges the two moves out of `scripts/node/serve.sh` into
`scripts/lib.sh`, so the launcher and the daemon share it and cannot drift apart. On the node,
`~/.config/localcode/endpoint` is removed, so the launcher uses its local default.

The reserve moves into each desk profile of the machine file. A headless node and a desk on
the same hardware keep different amounts back, and one top-level `reserve_gb` cannot hold
both. The role file names the profile (`DESK_PROFILE`). `cap.sh`, `gpuraise.sh` and
`rungs.sh` read that profile's reserve. The existing values stay as they are: the laptop's
8 and the node's headless 6. The GPU cap daemon stays on in the workstation role, with the
reserve of the `attended` profile.

The attended numbers are measured the way the laptop's were, and the laptop's are not
carried over. The idle reading is taken logged in, with the owner's editor and a browser
open, and the reserve is that reading plus 1 GiB. Next, the ladder runs under condition
`attended` from explicit `CELLS`, starting at 49,152. A rung passes when it fills with zero
swap, headroom at or above the floor, and `deskprobe.sh` showing that the compositor kept
drawing. The largest passing rung is the ceiling of the `attended` profile in
`config/machine-m5max-36gb.json`. The ceiling is proven by a chain of real work run on the
node while the desk is in use, not by the generated fixture.

VISION changes in this round: a Mac may switch between serving only and being the
developer's desk, and its role decides whether the desk rule applies to it.

Files: `cmd/localcode/main.go`, `scripts/lib.sh`, `scripts/node/serve.sh`, `scripts/node/cap.sh`,
`scripts/gpuraise.sh`, `scripts/rungs.sh`, `config/machine.json`,
`config/machine-m5max-36gb.json`, `config/roles/workstation.env`, `docs/TECH.md`.

## Tasks

- [ ] The launcher on the node serves `config/node.env` with `serve.env` applied, using the merge in `scripts/lib.sh` that `scripts/node/serve.sh` now calls, and stops the server when the session ends, covered by a test
- [ ] Each desk profile in the machine files carries its own `reserve_gb`. `cap.sh`, `gpuraise.sh` and `rungs.sh` read the reserve of the profile the role names, and the laptop's and the node's headless values are unchanged, covered by a test
- [ ] The node's idle reading as a workstation, with the owner's editor and a browser open, is recorded in TECH and sets the reserve of the `attended` profile
- [ ] The ladder runs on the node under condition `attended` from explicit `CELLS`. TECH records peak wired, headroom, swap and the compositor verdict for each rung, and the largest passing rung becomes the `attended` ceiling in `config/machine-m5max-36gb.json`
- [ ] A chain of real work completes on the node at the attended ceiling while the editor and browser are in use, as shown by the session's own rows

## Log
