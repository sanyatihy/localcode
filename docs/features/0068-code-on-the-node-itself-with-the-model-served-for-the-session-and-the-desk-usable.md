---
id: 0068
title: Code on the node itself, with the model served for the session and the desk usable
status: Draft
created: 2026-09-25
submitted:
needs: 0067
---

## Problem

The node is to become the owner's only machine. Its limits were measured with nobody logged
in: a 6 GB reserve and a 163,840-token ceiling. The serving daemon also keeps about 24 GB
wired from boot. Once an editor and a browser share the 36 GB, VISION's desk rule applies,
and nothing yet says which context the node can carry in attended use.

## Non-goals

- Deleting the link daemon or the remote-endpoint path. The GB10 (0054) still uses them.
- Changing `config/node.env`. If the attended ceiling is below its context, the lower value
  goes in the desk's own `~/.config/localcode/serve.env`, following 0064.

## Design

The model is served on demand. The workstation role has no serving daemon (0067), so the
launcher starts and stops the server for each session, as it did on the laptop, and the
memory is free when the owner is not coding. The launcher serves `config/node.env` with
`serve.env` applied. The merge moves from `scripts/node/serve.sh` into `scripts/lib.sh`, so
there is one copy.

The desk's limits go in a machine file of their own, `config/machine-m5max-36gb-desk.json`.
Its reserve, floor and `attended` ceiling are measured on the desk. A second file needs no
code change, because every script already reads the file `MACHINE` names. The headless file
stays as the record of the node. The workstation role installs the GPU cap daemon with the
desk file, which gives the cap a desk's reserve and not a node's.

The numbers are measured the way the laptop's were. The idle reading is taken logged in,
with the owner's editor and a browser open, and the reserve is that reading plus 1 GiB. The
ladder then runs under condition `attended` from 49,152. A rung passes when it fills with
zero swap, headroom at or above the floor, and a compositor that keeps drawing. The ceiling
is proven by a chain of real work run while the desk is in use.

Files: `cmd/localcode/main.go`, `scripts/lib.sh`, `scripts/node/serve.sh`,
`scripts/node/install.sh`, `config/machine-m5max-36gb-desk.json`, `docs/TECH.md`.

## Tasks

- [ ] The launcher serves `config/node.env` with `serve.env` applied, using the one merge in `scripts/lib.sh`, and stops the server when the session ends, covered by a test
- [ ] The workstation role installs the GPU cap daemon with `config/machine-m5max-36gb-desk.json`, and its reserve comes from the desk's idle reading with the editor and a browser open, recorded in TECH
- [ ] The ladder runs on the desk under condition `attended`, TECH records each rung, and the largest passing rung is the desk file's `attended` ceiling
- [ ] A chain of real work completes on the desk at that ceiling while the editor and browser are in use

## Log
