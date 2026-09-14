---
id: 0053
title: Drive a session against a model served on another machine
status: Draft
created: 2026-09-13
submitted:
needs:
---

## Problem

`localcode -endpoint` verifies health and the served context on the URL it is given,
then both launcher harnesses talk to `127.0.0.1:8081`: the endpoint is written into
the committed Claude Code environment and the Pi provider file, the sandbox admits
loopback only, and a missing server makes the launcher start one on the laptop. VISION
names a second Mac on a trusted network as a place to serve, and the GB10 (0054) uses
the same client path.

## Non-goals

- Several users on one endpoint: 0054.
- TLS or per-user authentication: VISION fixes the network as trusted.
- Reading a remote server's log from the client. `cmd/prefixlog` and the account report
  read a local `serve.log`; a remote log is read on the serving machine.
- Hermes and OpenCode. They are not launcher harnesses; their provider files in
  `harness/` are edited by hand and their READMEs say so.
- Porting the desktop and Metal probes. They run on the machine that serves.

## Design

The URL the launcher verified is the URL Claude Code is handed: the launcher sets
`ANTHROPIC_BASE_URL` after the committed file. The file keeps loopback as its default.
Pi keeps its provider file; a remote endpoint under Pi is refused naming the file, and
a file already naming that endpoint lifts the refusal.

An endpoint is loopback when its host is `localhost`, `127.0.0.0/8` or `::1`. Anything
else is remote. A URL that does not parse is refused before anything runs.

`~/.config/localcode/endpoint` holds the machine's default endpoint; `-endpoint` wins.
A file rather than an environment variable: the sandbox strips the session's
environment, and `writable` already lives there. `smoke.sh` keeps reading `ENDPOINT`;
README shows `ENDPOINT=... make smoke`.

A remote endpoint is never started or stopped from the client. A missing remote
server is refused naming the command to run on the serving machine; `localcode stop`
refuses. Neither refusal leaves a process behind.

The launcher resolves the endpoint's host at startup and writes one
`(allow network-outbound (remote ip "addr:port"))` line per address. Seatbelt matches
addresses, not names. `-net` is unchanged.

`serve.sh` takes `HOST` from the environment over the config and prints it in the
banner. Committed configs keep `HOST="127.0.0.1"`; a machine that serves the network
sets `HOST` in the environment of whatever starts the server.

The network's cost is one serving Mac on `config/agent.env`, driven from itself and
from the other Mac, cold prompt cache each time, three repetitions each. Compared:
chain wall, per-call latency from the transcript, and the server's own prompt and
decode rates. The serving Mac's desk profile applies; the client's does not.

## Tasks

- [x] Claude Code talks to the endpoint the launcher verified, including `count_tokens`, set after the committed file; a test covers a remote URL, and Pi refuses a remote endpoint its provider file does not name
- [x] A URL is classified loopback or remote including `::1`, an unparseable one is refused, and a test covers each case
- [x] A missing remote server is refused with the command to run there, `localcode stop` refuses a remote endpoint, and a test shows neither starts a process
- [x] The sandbox admits the endpoint's resolved addresses and port beyond loopback and nothing else, covered by a test on the written profile and by one session that reaches the endpoint and is denied another host
- [x] `~/.config/localcode/endpoint` sets the default endpoint, `-endpoint` wins, and `localcode status` reports which it used
- [x] `serve.sh` binds to `HOST` from the environment and prints it; README documents serving on one Mac and running `ENDPOINT=... make smoke` and `localcode` from the other
- [ ] One serving Mac is driven from itself and from the other Mac, three cold repetitions each, and TECH records chain wall, per-call latency and server rates side by side

## Log
- 2026-09-14 — paused: the Mac-to-Mac measurement needs a second machine and a model run
