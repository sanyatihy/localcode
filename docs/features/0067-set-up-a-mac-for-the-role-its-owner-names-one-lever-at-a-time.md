---
id: 0067
title: Set up a Mac for the role its owner names, one lever at a time
status: Draft
created: 2026-09-25
submitted:
needs:
---

## Problem

`scripts/node/` sets a Mac up as a dedicated node or not at all. A single variable,
`LOCALCODE_NODE=1`, applies every lever in `prepare.sh` and all three daemons in
`install.sh`. No lever can be kept or dropped on its own, and nothing turns a lever back on,
so the node cannot become the owner's workstation without work by hand, which 0056 ruled
out. The observable result is that one command sets any Mac to either role, and a second
run reports that nothing changed.

## Non-goals

- Apps, dotfiles and macOS preferences that serving does not need. These belong to the
  owner's private repository, which runs `bootstrap.sh` as one of its steps. This repository
  is public, and VISION limits it to how the model is launched, served and driven.
- A new engine such as nix-darwin or Ansible. The bash scripts already check state before
  acting and are proven on the node, and a rewrite would not change what they do.
- Renaming `scripts/node/`. README, TECH and seven feature docs cite that path, and
  renaming it changes nothing else.
- What the workstation serves and at which context: 0068.

## Design

Two committed role files, `config/roles/node.env` and `config/roles/workstation.env`,
set every lever in `KEY="value"` form. A lever is named after the thing it controls, and its
value says whether that thing runs: `SPOTLIGHT_INDEXING="off"` on a node and `"on"` on a
workstation. The daemons are levers too: `LINK_DAEMON`, `GPU_CAP_DAEMON` and `SERVE_DAEMON`.
The machine's own `~/.config/localcode/setup.env` names `ROLE` and may override any single
lever, following 0064: the repository holds the defaults and the machine holds its own
choices. A lever name that neither role file declares is refused, and so is a role file that
leaves one of the levers out. Either one would otherwise be a setting that silently does
nothing.

`ROLE` replaces `LOCALCODE_NODE=1`. A run with no role named refuses and names the file to
write, so the old gate's protection remains: nothing is applied to a machine by accident.
`install.sh --link-only` is retired. A machine that wants only the link sets
`LINK_DAEMON="on"` in its own file.

Every lever reads its current state, acts only when that state differs from the role's
value, and reports that it acted or that the state was already in place. Before this,
`prepare.sh` re-applied every lever on every run. Levers now converge in both directions.
Turning a lever back on restores macOS's default: where `prepare.sh` wrote a `defaults`
key, the key is deleted, not written with the opposite value. A daemon turned off is
booted out and its plist removed. `--check` reads every lever and daemon without changing
anything and exits 1 on drift. It is the plan step that IaC tools provide, and it is the
evidence that a machine matches its role.

Some levers are role-specific:
- On a workstation the firewall stays on and in stealth mode, but its allowance list is not
  managed. On a node the list is replaced with exactly sshd and the server, which would
  remove what the owner has allowed on their own desk.
- Remote Login is never turned off from inside an SSH session. The run refuses and names
  the lever, because the session making the change is the only way in.
- The idle reading runs only for the node role. It is compared with the machine file's
  record, and a desk has no record to compare against.

`bootstrap.sh` becomes the single command. After the checkout and the build it runs
`prepare.sh` and `install.sh` for the role, reusing the sudo credential it already holds.
Both scripts can still be run on their own. The role parser is one function in
`scripts/lib.sh`, so all three scripts read the same files the same way.

Files: `scripts/node/bootstrap.sh`, `scripts/node/prepare.sh`, `scripts/node/install.sh`,
`scripts/lib.sh`, `scripts/scripts_test.go`, `config/roles/node.env`,
`config/roles/workstation.env`, `README.md`, `docs/TECH.md`.

## Tasks

- [ ] `config/roles/node.env` and `config/roles/workstation.env` set every lever. `scripts/lib.sh` reads `ROLE` and per-lever overrides from `~/.config/localcode/setup.env`, and refuses an unknown lever or a role that leaves a lever out, covered by `scripts_test.go`
- [ ] Every `prepare.sh` lever reads its state, converges to the role's value in either direction and reports whether it acted. `--check` changes nothing and exits 1 on drift, and on the node as it stands it reports no drift against the node role
- [ ] `install.sh` installs the daemons the role turns on and boots out and removes the ones it turns off. `LOCALCODE_NODE=1` and `--link-only` are retired in favour of the role
- [ ] `bootstrap.sh` runs `prepare.sh` and `install.sh` for the role, and a second run reports every step as already in place
- [ ] The node is converted to the workstation role by `bootstrap.sh`. A second run and `--check` both report no change, and README and TECH describe how to set up a new Mac for either role

## Log
