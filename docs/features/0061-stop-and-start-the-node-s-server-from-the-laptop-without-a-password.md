---
id: 0061
title: Stop and start the node's server from the laptop without a password
status: Draft
created: 2026-09-19
submitted:
needs:
---

## Problem

The node's server is a system LaunchDaemon, so ending it is a `launchctl bootout` that
needs root, and 0056 decided the node carries no passwordless sudo. Stopping it from the
laptop is therefore an SSH session, a `cd`, a `sudo` password and `scripts/stop.sh`, and
starting it again is `scripts/node/install.sh` under `sudo` with two variables.
`localcode stop` with the node's endpoint refuses and says to go there. Every measurement
on the node begins by stopping the daemon's server, and 0060 adds a second runtime that
cannot load beside it.

## Non-goals

- A sudoers rule, however narrow. 0056's decision stands; this removes the need for root
  rather than granting it.
- A control port on the node. SSH by key is the node's one way in and already
  authenticates; a second listener is a second thing the firewall has to admit.
- Stopping an endpoint somebody else shares. The refusal stands for every endpoint the
  owner has not named an SSH route for, which is what 0055's team endpoint will be.
- Choosing what the node serves from the laptop. The daemon serves `config/node.env`;
  a different config is a measurement, run by hand with the daemon's server stopped.
- The link and cap daemons. Neither is ever stopped in normal use.

## Design

The serving daemon is gated on a switch file the serving user owns.
`scripts/node/com.localcode.serve.plist` replaces `KeepAlive`'s `SuccessfulExit` with
`PathState` on `__HOME__/.local/state/localcode/serve.on`: launchd keeps the server alive
while the file exists and leaves it down while it does not. Both keys cannot stay,
because launchd ORs them and a killed server would be restarted as a failure. Stopping
becomes removing the file and ending a process the serving user already owns; starting
becomes creating the file. Neither needs root, and the switch survives a reboot, so a
node stopped on purpose stays stopped.

launchd documents that `KeepAlive` implies a speculative launch at load, so the plist
runs `scripts/node/serve.sh`, which exits 0 when the switch is absent and otherwise execs
`scripts/serve.sh config/node.env`. The switch then decides at boot whatever launchd
does. `scripts/node/install.sh` creates the switch on a first install only, so an
upgrade does not restart a node that was stopped.

`stop_server` in `scripts/lib.sh` takes the switch route when the daemon is loaded and
the switch exists, keeps its wait for the memory back, and keeps the bootout for a node
whose installed plist predates this. `scripts/stop.sh` and `localcode stop` on the node
then work as the serving user. `localcode serve` on a machine whose daemon is loaded
creates the switch and waits for health instead of starting a second server.

From the laptop, `stop` and `serve` against a remote endpoint run the node's own
`~/.local/bin/localcode` over SSH, named by path because a non-login shell has no
`~/.zprofile`. The destination is read from `~/.config/localcode/ssh`, one line of
`user@host`, beside the `endpoint` file already there. With no such file the present
refusal stands, reworded to name the file. Naming a destination is the owner saying the
machine is theirs to stop; inferring it from the endpoint's host would stop a shared
server for anyone who could log in. SSH runs with `BatchMode=yes`, so a missing key is
an error and never a prompt. `status` is unchanged: it already reads the endpoint.

Changed: `cmd/localcode/main.go`, `scripts/lib.sh`, `scripts/stop.sh`,
`scripts/node/install.sh`, `scripts/node/com.localcode.serve.plist`, the new
`scripts/node/serve.sh`, `scripts/scripts_test.go`, README's node runbook and TECH's
daemon paragraph.

Done is observable from the laptop with no password typed: `localcode stop` returns
once the node's memory is back, `localcode status` says no server and still says so a
minute later, `localcode serve` returns with the model answering, and a node rebooted
while stopped comes up not serving.

## Tasks

- [x] `scripts/node/serve.sh` serves `config/node.env` when the switch exists and exits 0 when it does not, the plist keeps the job alive on `PathState` alone, and a test renders the plist and lints it with `plutil`
- [x] `scripts/node/install.sh` creates the switch on a first install only and replaces an installed plist that predates it, and a rerun restarts nothing
- [x] `stop_server` stops a switched daemon as the serving user and still waits for the memory back, the bootout route stays for an older plist, and tests cover both
- [x] `localcode serve` on a machine whose daemon is loaded creates the switch and waits for health, covered by a test
- [ ] `localcode stop` and `localcode serve` against a remote endpoint run the node's launcher over SSH to the destination in `~/.config/localcode/ssh`, refuse by name when that file is absent, and a test covers both with a stub `ssh`
- [ ] On the node: stop, a minute of `status`, serve, and a reboot while stopped are driven from the laptop with no password, and TECH records what launchd did at load with the switch absent
- [ ] README's node runbook gives the laptop commands and the `ssh` file, and TECH's daemon paragraph says what the switch is and why `SuccessfulExit` left
