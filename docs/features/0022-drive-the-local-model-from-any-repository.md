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
- **The sandbox is not a defence against a hostile model.** It is a blast radius for a
  mistaken one. Seatbelt is what macOS offers without a VM, and a determined escape is out
  of scope; `docs/VISION.md` has one developer on one machine, not an adversary.
- **No per-language knowledge.** The profile names no toolchain. An ecosystem that needs a
  path outside the default set is a line in the user's config, never a case in the tool.

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

**The agent runs under a seatbelt sandbox, and that is what makes broad tool access safe.**
`--allowedTools Bash` is unrestricted shell, and a pattern allowlist cannot be both complete
and generic — every ecosystem builds differently, and a denylist of dangerous strings is not
a boundary, since `sh -c` defeats string matching. `sandbox-exec` is a kernel boundary and
needs to know nothing about the language. It wraps `claude` itself, so every child process
inherits it.

**Writes are confined; reads are not.** The writable set is the working directory, the
resolved `TMPDIR`, `/private/tmp`, and the two standard cache roots — `~/Library/Caches` and
`~/.cache`. Reading stays unrestricted, because an agent that cannot read a toolchain cannot
use one. Measured under that profile: Go, Python, Node, `make` and `git` all complete, and
`rm -rf` outside it is refused by the kernel.

**Paths are resolved before they reach the profile.** `/var`, `/tmp` and `/etc` are symlinks
into `/private`, and seatbelt matches the resolved path — an unresolved `TMPDIR` denies every
compiler that uses one while appearing to allow it.

**A denied write fails loudly and is widened in one line.** The kernel names the path;
`~/.config/localcode/writable` is a list of extra subpaths, and the launcher prints the line
to add. That is what keeps the tool generic: it learns no ecosystem, and the developer
records the one their repository needs.

**The network is loopback-only by default.** The agent reaches the server on 127.0.0.1 and
nothing else, so a repository's source cannot leave the machine and `curl | sh` fetches
nothing. This is `docs/VISION.md`'s offline property enforced rather than configured.
`--net` allows outbound for one session and says so at startup; dependency installs and
`git push` are what it is for.

## Tasks

- [x] `cmd/localcode` runs the agent in the working directory against a server that is
      already up, with 0008's tools and permissions, writing nothing to that repository
- [x] `make install` puts it on `PATH` with this checkout's location stamped in, and
      `localcode status` reports what is served
- [x] the bare command starts a server when none is running, prints the cost first, and
      `--no-serve` refuses instead
- [x] `serve` and `stop` reach the same scripts `make serve` and `make stop` do
- [x] handoff works in any repository, with state under `~/.local/state/localcode/` and
      `HANDOFF.md` still at the checkout root when working on localcode itself
- [x] the agent runs under a seatbelt profile that confines writes to the working
      directory, temp and the cache roots, with reads unrestricted
- [ ] a write denied outside that set names the path and the line that would allow it, and
      `~/.config/localcode/writable` widens it
- [ ] the network is loopback-only by default and `--net` opens it for one session, both
      asserted against a real endpoint
- [ ] the README's manual is the installed command, and `harness/claude-code/README.md`
      says which of the two flows a reader wants

## Open questions

- Whether `kit` should own the handoff rule rather than this repository shipping one
  vendor's hooks. `docs/features/BACKLOG.md` carries it; this feature moves where the hooks
  can run and does not settle who owns them.

## Log

- **The tool scope grew a sandbox before any code was written.** The design had
  `--allowedTools Bash,Edit,Read,Write` and called the permission question settled, which is
  unrestricted shell on the developer's own machine. Three boxes were added rather than
  changing the four that existed, since the launcher is the same launcher either way.
- **A pattern allowlist and a denylist were both rejected before seatbelt was tried.** An
  allowlist of commands cannot be generic across ecosystems, and a denylist of dangerous
  strings is defeated by `sh -c`. Neither is a boundary; the kernel is.
- **The hook's prose was part of the mechanism.** Relocating the state was not enough: the
  SessionStart text told the model to create the handoff "at the root of the checkout", so
  it did, in the repository being visited. The instruction now names the path it wants.
