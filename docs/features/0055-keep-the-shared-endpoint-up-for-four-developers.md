---
id: 0055
title: Keep the shared endpoint up for four developers
status: Draft
created: 2026-09-13
submitted:
needs: 0054
---

## Problem

0054 makes the GB10 serve four slots and measures what that costs; it does not make the
endpoint something four people rely on. A config change restarts the server under
whoever is mid-session, one developer's subagents can take every slot, the server log
grows without bound, a reboot leaves nothing serving, and nothing tells the team the
endpoint is down before their next request fails. Each of those is a support request
the day it happens.

## Non-goals

- Authentication, quotas or billing per user. VISION fixes the network as trusted, and
  the slot count is the only fairness mechanism this adds.
- High availability. One box, one server; a failed box is down until it is fixed.
- A dashboard. The server already exposes `/metrics` and `/slots`; a page that reads
  them is a later feature if anyone wants one.

## Design

A restart drains before it stops. `stop.sh` on the box reads `/slots`, waits for the
busy ones to finish within a bound, and stops the unit when they are idle or the bound
runs out; the bound is a config setting and the wait is reported. A restart while a slot
decodes ends a user's session mid-turn, and the harness reports it as a failed call
rather than as a restart.

The server is enabled at boot, and its log is rotated. The unit is enabled, so a reboot
serves again without anyone logging in. journald holds the unit's output with the box's
own rotation, and the server log 0018 reads is a journalctl export rather than a file
that grows for months.

Slots are per user before they are per request. The launcher today lets Claude Code
send its background calls and subagents in parallel, all of which queue for the endpoint;
on one slot that was harmless and on four it lets one developer hold the other three.
What one session actually opens concurrently is measured from `/slots` under a real
chain first. If a single session takes more than one slot at a time, the launcher caps
it to one through the harness's own concurrency settings, and TECH records which
settings those are. A cap invented before the measurement is the kind of arithmetic
VISION says has been wrong three times.

Down is announced, not discovered. A check on the box hits `/health` on a timer and
posts to a channel the team reads when it changes state; which channel is a config
value. `localcode status` on a client already says when the endpoint is not there.

Each teammate installs once from the README. The two-Mac section 0053 writes covers
the client; this adds the per-user endpoint file and the check that `smoke` passes
from their Mac before their first session.

## Tasks

- [ ] `stop.sh` on the box drains busy slots within a configured bound before stopping the unit, and reports how long it waited
- [ ] The unit is enabled at boot and its output goes to journald; the log 0018's reader needs is exported from journalctl and TECH records the command
- [ ] One real chain is measured from `/slots` for how many slots a single session opens at once, and TECH records the number
- [ ] If a session opens more than one slot, the launcher caps it to one through the harness's concurrency settings, covered by a test on the assembled environment
- [ ] A timed `/health` check on the box announces a change of state to a configured channel
- [ ] README's client section takes a teammate from a fresh Mac to a passing `smoke` against the GB10 and a first session
