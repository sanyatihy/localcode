---
id: 0008
title: Wire the winning config into the coding agent
status: Draft
created: 2026-08-17
shipped:
check:
checked:
review:
needs: 0004
related: 0005
---

## Problem

A benchmark-winning endpoint that nobody codes against has not delivered the vision's
first outcome: a real coding agent completing real tasks locally. Harness tasks are
fixed and sanitised; a live agent sends longer contexts, messier tool schemas, and
multi-turn histories, which is where a config that looked good on paper tends to break.

## Non-goals

- **Not building an agent.** Configuration and verification of an existing tool only.
- **No further config tuning.** If live use contradicts the harness, that is a finding
  worth a new feature — and evidence the suite is unrepresentative — not a quiet re-tune.
- **No cloud fallback.** The vision rules out routing; an offline setup that silently
  reaches the network is the failure this project exists to avoid.

## Design

Point a real OpenAI-compatible coding tool at the local endpoint and complete a genuine
task in this repository, end to end. The endpoint's context limit is the thing most
likely to bite: agent harnesses assume large windows, and a 16–32k local ceiling will be
hit by file reads long before the model reasons badly. So the setup must make truncation
**visible and early** rather than letting the agent silently lose the head of its context.

Verification includes an explicit offline check — the vision's central claim is that no
code leaves the machine, and the only honest way to hold that is to sever the network and
watch the loop still work.

What live use reveals about context pressure is recorded even when it agrees with the
harness, because 0002's suite is the instrument everything else trusts, and the first
real workload is the only chance to see whether it measures the right thing.

## Tasks

- [ ] The coding tool is configured against the local endpoint from committed settings, no hand-typed flags
- [ ] A real task in this repo is completed end to end, and the transcript is recorded
- [ ] Context exhaustion is made visible: the setup surfaces truncation rather than silently dropping history
- [ ] The loop is verified with the network disabled, proving no cloud dependency
- [ ] Any divergence between live behaviour and harness predictions is written up, with a BACKLOG line or new feature if the suite needs to change

## Open questions

- Which agent to target first? Leaning whichever is already installed and speaks
  OpenAI-compatible — the point is a real loop, not tool advocacy.

## Log
