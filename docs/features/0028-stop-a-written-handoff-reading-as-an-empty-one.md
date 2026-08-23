---
id: 0028
title: Stop a written handoff reading as an empty one
status: Draft
created: 2026-08-23
shipped:
needs:
---

## Problem

The supervisor reads a session's next step out of the `**Next:**` line and nothing else,
but it reads only the remainder of that line. A handoff that puts the marker on its own
line and the step beneath it extracts as empty, and the chain then stops on its own
loop-detector: two such handoffs in a row compare equal, so the supervisor reports that a
session planned what the last one did. Measured on a seven-session chain, that halt cost a
session of an unspent bound and stranded uncommitted work in a claimed worktree.

## Non-goals

- No change to the format the system prompt asks for — the inline `**Next:** …` line stays
  canonical, and this widens what is read, not what is requested.
- No inference of the next step from anything but the marker. Guessing from the last
  paragraph is the wider door `usable` already refuses.
- No change to the word list `Done` accepts. The completion signal is unreadable here for
  the same reason the next step is, and the fix is upstream of both.

## Design

`Next` falls through to the lines below the marker when the marker's own line ends there.
It stops at the next `**Field:**` marker or the end of the handoff, and joins what it
found. The alternative was to demand the inline form in the prompt and refuse anything
else, which the Stop hook cannot enforce: it sees the file before the supervisor does and
would have to reject a handoff carrying a next step it could read.

`usable` is defined as `Next(handoff) != ""` above the size floor, replacing its own
substring test. One definition, because the two disagreeing is what made the failure
silent: the Stop hook let both sessions end on handoffs it called usable, and the
supervisor read no step in either.

The chain's repeat guard compares two next steps only when both are non-empty. A step the
supervisor could not read is not a step that was planned twice, and the guard's own
evidence is about sessions that wrote the same plan — not about sessions that wrote none.
This is the second line of defence, kept because the first one parses prose.

## Tasks

- [x] `Next` returns the step when the handoff writes it below the marker
- [ ] the Stop hook and the supervisor agree on what a usable handoff is
- [ ] the chain does not stop on two handoffs whose next step it could not read

## Log
