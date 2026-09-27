---
id: 0067
title: Name a Mac's localcode role in one file, and let a node become a desk
status: Draft
created: 2026-09-25
submitted:
needs:
---

## Problem

`scripts/node/` has one mode. `LOCALCODE_NODE=1` applies every lever and installs every
daemon, and nothing undoes them, so the node cannot become the owner's desk without work by
hand. The owner's `mac-iac` repository now sets up their Macs and runs localcode's
`bootstrap.sh` as a dependency, which also needs a way to say which role a machine has.
The observable result is that the node is released to the workstation role by one revert
and one `bootstrap.sh` run, and that a second run changes nothing.

## Non-goals

- Setting up a Mac. `mac-iac` owns apps, dotfiles and desk preferences.
- Checking state before each lever. Re-applying a lever is already harmless, and a node
  that is not prepared shows up in the idle reading.
- The desk's GPU cap and context: 0068.

## Design

`SETUP_ENV` names one file, by default `~/.config/localcode/setup.env`, holding
`ROLE="node"` or `ROLE="workstation"`, and optionally `SKIP_LEVERS` listing node levers to
leave alone. One parser in `scripts/lib.sh` reads it, and it refuses an unknown role or lever
name. `ROLE` replaces `LOCALCODE_NODE=1`. A run with no role refuses and names the file to
write.

The node role does what the scripts do today, minus any skipped levers. The workstation
role applies no lever and installs no daemon. `install.sh` removes a daemon that the role
does not install, so moving from node to workstation stops the link and the server. It
also retires `--link-only`, because the laptop that used it is being retired.

`prepare.sh --revert` undoes what the node levers wrote. It deletes the `defaults` keys they
set, re-enables Spotlight indexing, Bluetooth and the services they disabled, and sets
`disablesleep` back to 0. Remote Login, the sshd drop-in and the firewall stay as they are:
a desk wants them too, and changing them over SSH can lock the session out. The revert is
run once, when a node becomes a desk. The workstation role does not re-run it on every
pass, because after the revert those settings belong to whoever sets up the desk. Running
it a second time is harmless.

`bootstrap.sh` runs `prepare.sh` and `install.sh` for the role, reusing the sudo credential
it already holds. This is the interface `mac-iac` calls: `SETUP_ENV` and `LOCALCODE_REF` go
in, and it exits 0 when every step is in place, 1 when a step failed, and 2 when it refused
to run.

Files: `scripts/node/bootstrap.sh`, `scripts/node/prepare.sh`, `scripts/node/install.sh`,
`scripts/lib.sh`, `scripts/scripts_test.go`, `README.md`, `docs/TECH.md`.

## Tasks

- [ ] `scripts/lib.sh` reads `ROLE` and `SKIP_LEVERS` from the file `SETUP_ENV` names and refuses an unknown role or lever, covered by `scripts_test.go`
- [ ] `prepare.sh` applies the node levers not skipped, applies none on a workstation, and `--revert` undoes them as the Design says
- [ ] `install.sh` installs the role's daemons and removes the others. `LOCALCODE_NODE=1` and `--link-only` are retired
- [ ] `bootstrap.sh` runs `prepare.sh` and `install.sh` for the role without prompting, exits 0, 1 or 2, and a second run reports every step as already in place
- [ ] The node is reverted and bootstrapped as a workstation, a second run changes nothing, and README and TECH describe both roles

## Log
