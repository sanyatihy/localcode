---
id: 0044
title: Stand the sandbox between a credential and a commit
status: Draft
created: 2026-08-27
shipped:
needs:
---

## Problem

The sandbox denies the network and confines writes to the repository, and a repository is
a thing a human pushes. So the model may read `~/.ssh/id_rsa`, write it into the working
tree, and leave with the developer's own next commit — a path the kernel boundary does not
cross and nothing in the briefing mentions. Reads are open by a decision that was made
against curiosity, not against exfiltration.

## Non-goals

- **Not closing reads.** An agent that cannot read a toolchain cannot use one, which is
  why the policy opens them; this narrows a named list and leaves the decision standing.
- **Not a secret scanner.** Watching what a session writes for things that look like keys
  is a detector with false positives on a repository full of test fixtures. This removes
  the read instead.
- **Not protection against a session that is told to do this.** The developer can hand a
  session anything. What this stops is a repository's own content doing it unasked.

## Design

**The threat is the repository, not the network.** `-net` is off by default and says so
when it is on, and every write already lands somewhere the developer chose. That reads as
closed and is not: the working tree is the exfiltration channel, and it is the one channel
the design cannot deny, because writing to it is the work. The asymmetry is that the loud
risk is announced and the quiet one is not.

**A read deny-list over credential roots, after `(allow default)`.** Seatbelt takes
`(deny file-read* (subpath …))` following the allow, and it was measured: `cat` on a
denied path returns `Operation not permitted` while an ordinary read and `go version` both
still work. So the cost of this is a list, and the list is what has to be right rather than
long — `~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.config/gh`, `~/Library/Keychains`. Each is a
directory whose only contents are credentials, which is what makes denying it safe; a
directory that also holds configuration a build reads is not a candidate.

**Denied, not hidden.** The list goes in the briefing beside the writable roots, for the
reason the writable roots are there: a refusal a session cannot explain becomes a session
working around it, and this one has an honest answer — the path holds credentials and the
work does not need them.

**Widening a policy says what it opened.** `~/.config/localcode/writable` is read, expanded
and applied in silence, so a line reading `/` opens the filesystem and nothing prints. `-net`
prints `network: outbound ENABLED for this session` for a smaller hole. The same sentence is
owed here, naming each path, and a line that widens a *denied* root has to be refused rather
than announced — otherwise the deny-list is advisory.

## Tasks

- [x] A session cannot read the credential roots, and can still read a toolchain, a
      repository and its own caches
- [ ] The session is told which roots are denied and why, in the briefing that already
      names what it may write
- [ ] Widening the writable list prints what it opened, and a line that would re-open a
      denied root is refused rather than applied

## Open questions

- **`.env` files inside the repository being worked on.** They are the commonest real
  secret on a developer's disk and the one credential root that cannot be denied by path,
  because it sits inside the directory the work happens in. Leaning: out of scope here and
  a `BACKLOG.md` line — denying a read inside the working tree breaks the case where fixing
  the `.env` *is* the task, and the honest mitigation is that the file is already in the
  repository the human pushes.

## Log
