---
id: 0055
title: Keep the shared endpoint up for four developers
status: Draft
created: 2026-09-13
submitted:
needs: 0053, 0054
---

## Problem

0054 makes the GB10 serve four slots; it does not make the endpoint something four
people rely on. A config change restarts the server under whoever is mid-session, one
session's parallel calls can hold several slots, the log grows without bound, nothing
says how long a reboot takes to serve again, and nobody learns the endpoint is down
before a request fails.

## Non-goals

- Per-user authentication, quotas or billing: VISION fixes the network as trusted.
- High availability. One box; a failed box is down until fixed.
- A dashboard. `/metrics` and `/slots` exist; a page over them is a later feature.
- A per-developer slot cap. Slots are per request in llama-server and four sessions
  from one developer take four slots; sharing is best effort and README says so.

## Design

`stop_server` on the box reads `/slots`, waits for busy slots within a configured bound,
then stops the unit and reports how long it waited. llama-server has no admission
control, so a request arriving during the wait is cut when the bound expires; the
bound is short and the client retries.

The unit is enabled at boot with a configured restart backoff. Its output goes to
journald with retention set in the unit's drop-in; the log 0018's reader needs is a
journalctl export, and a test reads an exported log.

Claude Code's Explore, Plan and cron are already disabled in its environment. What
remains concurrent, background model calls above all, is measured from `/slots` under
a real chain. If a session opens more than one slot at once, the launcher caps it to
one through a documented Claude Code setting, and a contention test with two sessions
shows the cap holds.

The health check runs from a client Mac, not from the box: a checker on the box cannot
announce the box losing power or network. It polls `/health` at a configured interval
with a timeout and a startup grace, posts a change of state in either direction to a
configured channel, and a test cuts the box off and sees the post.

README's client section points at 0053's endpoint file and takes a teammate from a
fresh Mac to a passing `smoke` against the GB10 and a first session.

## Tasks

- [ ] `stop_server` on the box drains busy slots within a configured bound before stopping the unit, and reports how long it waited
- [ ] The unit is enabled at boot with restart backoff and journald retention set; a reboot is timed to serving and the number recorded in TECH; the log 0018's reader needs is exported from journalctl and a test reads the export
- [ ] One real chain is measured from `/slots` for how many slots a single session opens at once, and TECH records the number
- [ ] If a session opens more than one slot, the launcher caps it to one through a documented Claude Code setting, and a contention test with two sessions shows it holds
- [ ] A health check on a client Mac polls `/health` with a configured interval, timeout and grace, posts a change of state either way to a configured channel, and a test cuts the box off and sees the post
- [ ] README's client section takes a teammate from a fresh Mac to a passing `smoke` against the GB10 and a first session, and says sharing is best effort
