---
id: 0022
title: Drive the local model from any repository
status: Draft        # Draft | Shipped | Dropped — kit ship and kit drop write it
created: 2026-08-21
shipped:             # written by kit ship, never by hand
needs:
---

## Problem

Everything this project settled is reachable only from this checkout. `make serve` needs
the working directory, the README's shell function needs a hand-edited absolute path in
`~/.zshrc`, and 0016's handoff hooks address `$CLAUDE_PROJECT_DIR` — so in another
repository they resolve to scripts that are not there and the session dies telling you so.
The grind the project exists to move on-device happens in the repositories the developer
actually works in, and none of them is this one.

## Non-goals

- **Not a second harness.** `docs/VISION.md` rules one out; this launches Claude Code with
  the flags 0008 settled and adds no agent of its own.
- **Not multi-user and not a LAN service.** One machine, one developer, loopback only.
- **No new measurement.** 0008's `--tools` figures and 0016's handoff mechanism are taken as
  they were measured; this changes where they can be invoked from, not what they cost.
- **Not a second home for the serving config.** `config/*.env` stays the record of how a
  server was launched, and the launcher reads one rather than carrying flags.

## Design

**One command on `PATH`, installed rather than sourced.** `cmd/localcode`, built and
symlinked into `~/.local/bin` by `make install`. It beat a shell function in `~/.zshrc`,
which makes every user paste an absolute path that is wrong on any other machine, and Go is
what `docs/VISION.md` names for anything built.

**Four verbs, and the bare command is the common one.** `localcode` runs the agent in the
working directory; `serve`, `stop` and `status` are the server. `stop` calls
`scripts/stop.sh`, so the wait for the memory back has one home.

**The bare command starts a server if none is running.** A junior arriving at this project
should type one word, and requiring two terminals is the headache. It prints what it is
doing and what it costs before it does it, because ~17 GB and twenty seconds are not
something to discover afterwards. `--no-serve` refuses instead, which is what a script wants.

**The launcher knows where this checkout is because installation records it.** `make
install` stamps the path in at build time; nothing searches for it and no environment
variable has to be set. Reinstalling after moving the checkout is the documented fix.

**Handoff state lives outside the repository being worked in**, at
`~/.local/state/localcode/<slug>/HANDOFF.md`, keyed by the repository's path. 0016 wrote it
to the checkout root, which is right for this repository and wrong for somebody else's: it
has no `.gitignore` line for it, so the file lands as untracked noise in a tree the
developer did not ask us to touch. The hooks gain a `LOCALCODE_HANDOFF_DIR` override and
keep `$CLAUDE_PROJECT_DIR` as the default, so working on localcode itself is unchanged.

**The hooks are addressed absolutely.** `hooks.json` stays as it is for the two readers 0016
built it for; the launcher generates its own settings document with the installed path
baked in, which is the same reason the path is stamped at build time.

**Nothing is written to the repository being worked in.** `--allowedTools` pre-approves the
same four tools `--tools` exposes, so no permission is recorded there either. That property
is the point of the feature and is asserted in a test rather than described here.

## Tasks

- [ ] `cmd/localcode` runs the agent in the working directory against a server that is
      already up, with 0008's tools and permissions, writing nothing to that repository
- [ ] `make install` puts it on `PATH` with this checkout's location stamped in, and
      `localcode status` reports what is served
- [ ] the bare command starts a server when none is running, prints the cost first, and
      `--no-serve` refuses instead
- [ ] `serve` and `stop` reach the same scripts `make serve` and `make stop` do
- [ ] handoff works in any repository, with state under `~/.local/state/localcode/` and
      `HANDOFF.md` still at the checkout root when working on localcode itself
- [ ] the README's manual is the installed command, and `harness/claude-code/README.md`
      says which of the two flows a reader wants

## Open questions

- Whether `kit` should own the handoff rule rather than this repository shipping one
  vendor's hooks. `docs/features/BACKLOG.md` carries it; this feature moves where the hooks
  can run and does not settle who owns them.

## Log
