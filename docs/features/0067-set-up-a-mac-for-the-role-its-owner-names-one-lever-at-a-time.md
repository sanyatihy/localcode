---
id: 0067
title: Apply or release localcode's own levers by role, callable from another repository
status: Draft
created: 2026-09-25
submitted:
needs:
---

## Problem

`scripts/node/` applies every lever in `prepare.sh` and all three daemons in `install.sh`
behind a single variable, `LOCALCODE_NODE=1`. Nothing can undo them, so the node cannot stop
being a node without work by hand, which 0056 ruled out. The owner's `mac-iac` repository
now sets up their Macs and installs localcode as a dependency. For that to work, localcode
has to manage only what it applied itself, through an interface another repository can
call. The observable result is that `bootstrap.sh` puts a Mac in the node role or releases
it, a second run reports that nothing changed, and `--check` exits 0 on a machine that
matches its role.

## Non-goals

- Setting up a Mac: apps, dotfiles, macOS preferences, and the Command Line Tools and
  Homebrew as a machine's own tools. `mac-iac` does that. `bootstrap.sh` keeps its existing
  check-before-install steps for those two, so a reader without `mac-iac` can still
  reproduce the node. Under `mac-iac` both steps find them already in place.
- Choosing a desk's preferences for anything a lever touches, such as sleep, Bluetooth,
  Spotlight or the firewall. A released lever belongs to whoever sets up the desk.
- A new engine or a rename of `scripts/node/`. Neither changes what the scripts do.
- What the workstation serves and at which context: 0068.

## Design

Each lever has two values. `apply` puts the node's setting in force. `release` undoes what
localcode applied and then stops managing that setting. Two role files set every lever:
- `config/roles/node.env` applies all of them.
- `config/roles/workstation.env` releases every OS lever, the link daemon and the serving
  daemon, and applies the GPU cap daemon, which 0068 needs.

`SETUP_ENV` names the machine's own file, which defaults to `~/.config/localcode/setup.env`.
That file names `ROLE` and may override any single lever, following 0064. A lever name that
no role declares is refused, and so is a role file that leaves a lever out.

A third value that restores macOS's defaults was rejected. It would make localcode the owner
of a desk's preferences, which `mac-iac` or the owner set, and each would undo the other's
changes on every run.

Release needs to know what localcode applied. `~/.local/state/localcode/levers` records each
lever when it applies, and a release acts only on a lever recorded there. A second release
therefore finds nothing to do. A preference the desk's owner set later is never touched,
because it was never recorded.

What each release does is listed per lever in TECH:
- Where the lever wrote a `defaults` key, the key is deleted.
- Services localcode disabled are re-enabled.
- `disablesleep` goes back to 0.
- A daemon's plist is booted out and removed.
- The firewall and Remote Login are handed over exactly as they stand. Changing either one
  on release could lock out the session doing the release, so neither is touched.

Every applied lever reads its state, acts only on a difference, and reports whether it
acted. `--check` changes nothing and exits 1 on drift. For an applied lever, drift means its
value is not in force. For a released lever, drift means localcode still records it as
applied.

`bootstrap.sh` installs its packages from `scripts/node/Brewfile` with `brew bundle`: go,
python, llama.cpp and `claude-code@latest`. Before this they were a list inside the script.
The file is the contract with `mac-iac`, whose engine reads it and refuses a module that
declares the same package, so each package has one owner. The move away from the lagging
`claude-code` cask stays a step of its own.

The lines `bootstrap.sh` appends to `~/.zprofile` (Homebrew's shellenv and
`~/.local/bin` on `PATH`) become the lever `SHELL_PROFILE`. `mac-iac` releases it because
its own profile carries those lines. A profile that `mac-iac` links from its checkout would
otherwise gain an append on every run.

`ROLE` replaces `LOCALCODE_NODE=1`. A run with no role refuses and names the file to write.
`install.sh --link-only` is retired, and a machine that wants only the link overrides that
one lever. After the build, `bootstrap.sh` runs `prepare.sh` and `install.sh` for the role,
reusing the sudo credential it holds. Both scripts can still be run on their own, and all
three read the role through one parser in `scripts/lib.sh`.

The interface another repository calls is `bootstrap.sh` with `SETUP_ENV` and
`LOCALCODE_REF`. With sudo cached it runs without prompting. It exits 0 when every step is in
place, 1 when a lever did not apply or release, and 2 when it refused.

Files: `scripts/node/bootstrap.sh`, `scripts/node/prepare.sh`, `scripts/node/install.sh`,
`scripts/node/Brewfile`, `scripts/lib.sh`, `scripts/scripts_test.go`,
`config/roles/node.env`, `config/roles/workstation.env`, `README.md`, `docs/TECH.md`.

## Tasks

- [ ] `config/roles/node.env` and `config/roles/workstation.env` set every lever to apply or release. `scripts/lib.sh` reads `ROLE` and per-lever overrides from the file `SETUP_ENV` names, and refuses an unknown lever or a role that leaves one out, covered by `scripts_test.go`
- [ ] Every `prepare.sh` lever records what it applies and releases only what it recorded, as TECH's per-lever table says. `--check` changes nothing and exits 1 on drift, and on the node as it stands it reports no drift against the node role
- [ ] `install.sh` installs the daemons the role applies and boots out and removes the ones it releases. `LOCALCODE_NODE=1` and `--link-only` are retired in favour of the role
- [ ] `bootstrap.sh` installs from `scripts/node/Brewfile`, touches `~/.zprofile` only while `SHELL_PROFILE` is applied, runs `prepare.sh` and `install.sh` for the role without prompting, and exits 0, 1 or 2 as the Design says. A second run reports every step as already in place
- [ ] The node is released to the workstation role by `bootstrap.sh`. A second run and `--check` report no change, and README and TECH describe the two roles and the interface

## Log
