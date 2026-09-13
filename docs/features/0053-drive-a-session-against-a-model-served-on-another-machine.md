---
id: 0053
title: Drive a session against a model served on another machine
status: Draft
created: 2026-09-13
submitted:
needs:
---

## Problem

`localcode -endpoint` verifies health and reads the served context from the URL it is
given, and then every harness talks to `127.0.0.1:8081` anyway: the endpoint lives in the
committed environment and provider files, the sandbox admits loopback and nothing else,
and a missing server makes the launcher start one on the laptop. VISION now names a
second Mac on a trusted network as a place the grind should happen, and it is the same
client path the GB10 (0054) needs.

## Non-goals

- Serving several users from one endpoint — 0054, where the slot count and its cost are
  measured.
- TLS or per-user authentication — VISION fixes the network as trusted and the traffic
  as plain HTTP.
- Reading the serving machine's log from the client. `cmd/prefixlog` and the account
  report keep reading a local `serve.log`; a remote server's log stays where it was
  written and is read there.
- Porting the desktop and Metal probes. They measure the machine that serves, and they
  run there.

## Design

The endpoint has one home per session, and it is the launcher's. The URL the
launcher verified `/health` and `/props` on is the URL the harness is handed: for Claude
Code the launcher sets `ANTHROPIC_BASE_URL` after the committed file rather than
letting the file set it. The committed file keeps loopback as its documented default, so
sourcing it by hand still works. Pi, Hermes and OpenCode keep a provider file each with
the endpoint written into it; a remote endpoint under those harnesses is refused with
the file to edit named. Refusing beats silently driving loopback, which is what happens
today, and following the launcher becomes a box when somebody runs Pi remotely — 0040
settled Pi as a candidate driver, and its provider file is one edit.

The default endpoint is set once per machine. `~/.config/localcode/endpoint` holds
one URL, read when `-endpoint` is not given; the flag still wins. A file rather than an
environment variable, because `writable` already lives there and the sandbox strips the
environment a session inherits.

A remote endpoint is never started or stopped from the client. The host of the URL
decides: loopback keeps today's behaviour, anything else makes a missing server a
refusal naming the command to run on the serving machine, and makes `localcode stop`
a refusal. The alternative, an SSH-driven start, puts a credential in the launcher's
path and is exactly what the sandbox exists to keep out.

The sandbox admits the endpoint and nothing else beyond loopback. The launcher
resolves the endpoint's host to its addresses at startup and writes one
`(allow network-outbound (remote ip "addr:port"))` line per address. Resolved by the
launcher rather than by name in the profile, because seatbelt matches addresses and a
name that resolves differently inside the sandbox admits nothing. `-net` is unchanged.

The serving machine binds to the network from the environment, not from a second
config. `serve.sh` takes `HOST` from the environment over the config when set, and
prints it in the banner. The bind address says where the server listens and not how it
serves, so it is the one setting that does not change what a measurement was of; a
second copy of `agent.env` differing in one line is the drift the config layout exists
to prevent. The config's own `HOST` stays `127.0.0.1`, so nothing binds outward by
accident.

What the network adds is measured, not assumed. One chain on the fixture is driven
Mac to Mac under the committed config and read beside the same chain driven locally.
The hypothesis is that a LAN round trip is invisible against decode, and it stays a
hypothesis until the rows say so. The serving Mac's own desk profile applies to it; the
client's does not.

## Tasks

- [ ] The endpoint the launcher verified is the endpoint Claude Code talks to, set from the launcher after the committed file; a test on the assembled environment covers a remote URL, and Pi, Hermes and OpenCode refuse a remote endpoint naming the provider file to edit
- [ ] A remote endpoint is never started or stopped from the client: a missing remote server is refused with the command to run there, and `localcode stop` refuses a remote endpoint
- [ ] The sandbox admits the endpoint's resolved addresses and port beyond loopback and nothing else, covered by a test on the written profile and by one session on the machine with the network otherwise denied
- [ ] `~/.config/localcode/endpoint` sets the default endpoint per machine and `-endpoint` still wins, with `localcode status` reporting which one it used
- [ ] `serve.sh` binds to `HOST` from the environment when set and prints it in the banner; README documents the two-Mac setup — serve on one, smoke and `localcode` from the other
- [ ] One chain on the fixture runs Mac to Mac and TECH records what the network added beside the local row
