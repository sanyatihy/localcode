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

## 2. Attended ceiling (your normal desktop)

Open what you normally work with — browser, editor, Spotify — then:

    CONDITION=attended ./scripts/ladder.sh

Same cells, same instrument, different machine state. `CONDITION` is what keeps the
two apart in the results file; without it they would average into one ceiling that
describes neither.

## Why the earlier attended run does not count

It ran before the ladder recorded fill duration, and duration is the metric that
discriminates: under saturation `ps rss` is clamped by what physically fits rather
than by what the config wants, so it stopped telling configs apart exactly where the
answer mattered. That run is kept outside the repo as
`.localcode-preboot/ceiling-attended-no-timing.jsonl`.

## Then

`docs/features/0003-*.md` boxes get ticked from these two runs, and 0004's per-profile
gates finally have real numbers: attended sweeps under the working ceiling, unattended
under the hard one.
