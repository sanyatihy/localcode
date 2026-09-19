---
id: 0063
title: Serve Splash from the node as a runtime the owner selects
status: Draft
created: 2026-09-20
submitted:
needs: 0062
---

## Problem

Running Splash today is an SSH session, a stopped daemon, a serve script started by hand
and a tunnel, and a reboot undoes all of it. The daemon, `stop_server` and the node's
firewall know one runtime. If 0062 finds Splash carries real work, the owner needs it the
way llama.cpp is there now: up at boot, stopped and started from the laptop, reached at the
address every tool already uses, and one command away from the runtime it replaced.

## Non-goals

- Making Splash the default. With no choice recorded the node serves llama.cpp, and
  flipping the default is one line after the owner has worked on it.
- Removing llama.cpp. It stays installed as the fallback, the laptop's only runtime and
  the baseline every measurement is read against.
- Running Splash's internal server directly for its `--host` flag. The vendor's launcher
  is the supported entry and hides that flag; bypassing it is a fork to maintain.
- The laptop, the GB10, a second Splash model, several slots.

## Design

One daemon and one switch, as 0061 left them. `scripts/node/serve.sh` reads the runtime
from `~/.local/state/localcode/runtime` beside the switch, `llama` when the file is absent,
and execs that runtime's serve script. Two daemons were rejected: the node cannot hold both
models, and exclusivity between two launchd jobs is a lock this design does not need. The
installed plist does not change, so adopting this needs no `sudo` on the node.

Splash binds `127.0.0.1:8000` and nothing else. The wrapper therefore runs `localcode
forward` beside it, a TCP forwarder in `cmd/localcode` from `0.0.0.0:8081` to it, and
passes the node's addresses to Splash as `--allowed-host`. The endpoint stays
`http://<node>:8081` for both runtimes, so `smoke`, the scorer, `chainrun.sh`, `status`
and 0053's proxy work unchanged. An SSH forward opened by the laptop's launcher was the
alternative: it needs no listener and authenticates, but every tool that is not the
launcher would need a tunnel of its own, and the exposure it avoids is the one llama.cpp
already has on a network VISION calls trusted. The forwarder is a second binary the node's
firewall has to admit, so `scripts/node/prepare.sh`'s firewall lever adds it, which is the
one step here that needs the owner's `sudo`. 0062's chain says whether Splash checks the
Host header's port; if `--allowed-host` does not cover it the forwarder rewrites the
header, and that is the only thing it would ever touch.

`stop_server` ends whichever runtime is up: `PROC` and the stop command cover
`llama-server`, Splash's server and its engine, and the forwarder ends with the wrapper.
`localcode serve -runtime llama|splash` records the choice, stops what is serving, waits
for the memory back and starts the other, locally or over 0061's SSH route, and refuses a
name it does not know. `localcode status` names the runtime. The choice survives a reboot
because it is a file, as the switch does.

Thinking is held off where the harness is configured and not per run:
`harness/claude-code/claude-code.env` sets `MAX_THINKING_TOKENS="0"`, which llama.cpp
ignores and without which 0060 read Splash slower than llama.cpp. `internal/harness/agent_pi.go`
takes the served context from `/status` where `/props` answers 404, as the launcher does.

The node stays infrastructure as code. `scripts/node/bootstrap.sh` runs
`runtimes/splash/setup.sh` and pins the formula with `brew pin`, since an upgrade moves
the version every row is tied to. The weights cannot be fetched on the node, whose route
to the CDN stalls, so `runtimes/splash/stage.sh` is run from the laptop: it downloads the
package at a named revision, copies it with `rsync --copy-unsafe-links`, and verifies
count and bytes on the node. README's node runbook gains both.

Changed: `scripts/node/serve.sh`, `scripts/lib.sh`, `scripts/node/prepare.sh`,
`scripts/node/bootstrap.sh`, `cmd/localcode/main.go`, `internal/harness/agent_pi.go`,
`harness/claude-code/claude-code.env`, `runtimes/splash/serve.sh`,
`runtimes/splash/stage.sh`, their tests, README and TECH.

Done is observable from the laptop: `localcode serve -runtime splash` returns with Splash
answering at the node's usual endpoint, `make smoke` and a chain pass against it
unchanged, `localcode stop` leaves nothing of it running, a reboot brings the same runtime
back, and `localcode serve -runtime llama` returns the node to what it serves today.

## Tasks

- [ ] `scripts/node/serve.sh` serves the runtime the state file names and llama.cpp when there is none, and `stop_server` ends either runtime and waits for the memory back, covered by tests
- [ ] `localcode forward` carries `0.0.0.0:8081` to Splash, the wrapper runs it with `--allowed-host`, and a test drives a request through it to a stub
- [ ] `localcode serve -runtime` records the choice and switches runtimes locally and over SSH, refuses an unknown name, and `localcode status` names the runtime, covered by tests
- [ ] `harness/claude-code/claude-code.env` holds thinking off and the Pi harness reads the context from `/status` where there is no `/props`, covered by tests
- [ ] `scripts/node/bootstrap.sh` installs and pins Splash, `runtimes/splash/stage.sh` stages the weights from the laptop at a named revision and verifies them, and `prepare.sh`'s firewall lever admits the forwarder
- [ ] On the node, driven from the laptop: switch to Splash, `smoke` and one chain through the usual endpoint, stop, reboot into the same runtime, and switch back, with TECH recording each
- [ ] README's node runbook covers choosing a runtime, staging weights and what needs `sudo`, and TECH's daemon paragraph says how one daemon serves two runtimes
