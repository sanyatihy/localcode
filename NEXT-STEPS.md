# After the reboot

Run these **before opening any apps** — the point of the reboot is a machine with
nothing else on it, and opening a browser first spends the measurement.

## 1. Unattended ceiling (clean machine)

    cd ~/Devel/github.com/sanyatihy/localcode-0003
    CONDITION=unattended ./scripts/ladder.sh

Five cells: 8k/16k/32k at q8_0, 16k at f16, 32k at q4_0. Each stops the server,
starts it on that config, verifies the server reports the context it was asked for,
fills the context genuinely full, and records memory plus **time to ingest**.

Takes 30-50 min and needs no supervision. Results append to `results/ceiling.jsonl`.

## 2. Read the unattended numbers before deciding to run anything else

If the timing spread across cells is flat, timing does not discriminate either and a
third run buys nothing. If it is wide, it is the metric, and an attended run on the
same instrument is worth the 30-50 minutes. Decide from the data rather than in
advance.

## 3. Attended, only if step 2 says so

Open what you normally work with, then:

    CONDITION=attended-fresh ./scripts/ladder.sh

Note the label. A machine rebooted minutes ago with apps just opened is **not** the
same condition as one that has been worked on all day, and calling both "attended"
would merge them into a ceiling describing neither.

## The three conditions, and why all of them are real

| Condition | Machine state | What it bounds |
|---|---|---|
| `unattended` | fresh boot, nothing else running | the hard ceiling — the grind profile |
| `attended-fresh` | fresh boot, apps just opened | best-case interactive use |
| `attended-worked-in` | hours of uptime, swap already allocated | interactive use as it actually is by afternoon |

`attended-worked-in` is already collected (5 cells, all completing a full-context
request) and **cannot be reproduced after a reboot** — that is precisely what a reboot
destroys. It predates the fill-duration metric, so it carries `"instrument":
"pre-timing"` and its outcome column is comparable while its timings do not exist.
Keeping it is why the reboot does not throw away the most realistic reading taken so
far.

## Then

`docs/features/0003-*.md` boxes get ticked from these two runs, and 0004's per-profile
gates finally have real numbers: attended sweeps under the working ceiling, unattended
under the hard one.
