---
id: 0008
title: Wire the winning config into the coding agent
status: Draft
created: 2026-08-17
shipped:
check:
checked:
review:
needs: 0005
related: 0010, 0005
---

## Problem

The working flow today is Claude Code driven from the Cursor/VSCode extension against a
hosted model. The goal is that exact flow with the local endpoint underneath — same
editor, same agent, same habits, different backend. Until that runs, the project has
produced benchmark numbers and no change to how work actually gets done.

## Non-goals

- **Not Cursor's built-in assistant.** Rejected in the design below on architecture, not
  preference. It is recorded there so nobody re-opens it.
- **No CLI harness comparison.** 0010 settles Pi, Hermes and OpenCode; this is the editor.
- **No further config tuning.** If live use contradicts the scorer, that is a finding and
  evidence the suite is unrepresentative, not a quiet re-tune.
- **No proxy or translation layer.** The design below shows none is needed; adding one
  would insert a component nobody measured between the agent and the server.

## Design

**The extension runs the agent on this machine.** Claude Code executes locally and the
editor hosts it, so the editor's own backend is never in the request path. That is what
makes this flow compatible with the vision, and it is the whole reason it beats the
alternative below.

**No proxy is needed.** The installed `llama-server` (build 10450) already serves
`/v1/messages` and `/v1/messages/count_tokens` — the Anthropic Messages API — converting
to its chat-completions path internally. Confirmed in the shipped binary, not inferred
from release notes. So `ANTHROPIC_BASE_URL` pointed at the local server is the entire
integration, and the LiteLLM / claude-code-router layer that most write-ups reach for is
dead weight here.

**Rejected: Cursor's built-in assistant.** It does not call the configured base URL from
this machine; it routes model requests through its own backend and rejects plain HTTP, so
a local model requires a public HTTPS tunnel and the request path becomes Cursor → its
backend → tunnel → here. Code leaves the machine even though inference does not, and it
adds a round trip per turn. The extension flow gets the same editor without any of that.

Three risks, in the order they will bite:

1. **Background model calls.** Claude Code makes non-essential calls — conversation
   titles, small-model checks — beyond the main completion. Against a single-slot local
   server these contend with or block the real request. The exact environment variables
   that redirect or disable them must be read from Claude Code's own documentation at
   build time rather than copied from a blog post, and this is task one because
   everything else is unusable until it is settled.
2. **Context ceiling.** Claude Code assumes a large window; 0003's measured ceiling is far
   smaller. Auto-compaction behaviour at a 16–32k limit is the thing most likely to make
   the flow feel broken, and it must be made visible rather than silently truncating.
3. **Translation fidelity.** Tool calls now cross an Anthropic→OpenAI conversion inside
   the server. That is a new surface 0005 never measured, so tool-call validity is checked
   through this path specifically, not assumed to carry over.

**This flow is not offline, and cannot be made so.** Claude Code's documented network
requirements include `platform.claude.com` for OAuth token exchange and refresh, and
`api.anthropic.com` for feature-flag fetches — and the fast-mode availability check is
documented as calling `api.anthropic.com` rather than the configured base URL. Telemetry
to the Datadog intakes is optional and switches off with
`CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC`; authentication does not.

So this feature delivers **local inference, not an offline harness** — the vision's weaker
property. That is an honest and probably acceptable outcome, since no prompt or file
content reaches a remote model. What it may not do is claim the stronger one. The offline
proof belongs to a harness that needs no vendor reachability, which is 0010's business.

## Tasks

- [x] Claude Code's background/small-model calls are characterised from its own documentation, and pointed at the local endpoint or disabled, with the exact variables committed
- [x] The half of the winning config that is per-request — thinking off and its sampling — is served as a default, since Claude Code sends neither
- [x] `ANTHROPIC_BASE_URL` against the local `/v1/messages` completes a real task in this repo from the terminal, with no proxy
- [ ] The same works driven from the Cursor/VSCode extension, matching the current flow, and the transcript is recorded
- [ ] Tool-call validity through the Anthropic→OpenAI conversion is measured and compared against 0005's numbers on the native path
- [ ] Context exhaustion and auto-compaction at the measured ceiling are made visible rather than silent
- [ ] The residual traffic is measured, not taken from the docs: what hosts the flow contacts during a real session, and which stop when non-essential traffic is disabled
- [ ] The failure mode when Anthropic hosts are unreachable is recorded — whether the session degrades, blocks, or refuses to start — since that is what an outage or a flight actually looks like
- [ ] The setup is committed as configuration, and `docs/TECH.md` records it, the residual traffic, and the rejection of Cursor's built-in assistant with its reason

## Open questions

- Does an API-key credential via `ANTHROPIC_AUTH_TOKEN` avoid the OAuth refresh path, and
  if so does anything besides feature flags still require reachability? Leaning: **it
  narrows the traffic but does not eliminate it**, which is why the task measures observed
  hosts rather than reasoning from the documentation.

## Log
- 2026-08-19 — **Claude Code becomes one harness among four rather than the destination.**
  Its configuration moves to `harness/claude-code/` beside the other three, and
  `internal/harness` gains an adapter so `cmd/tier2` drives it on the same terms; it
  passes `patch-nil-check` against the unseen test in 59.8 s. What the overhead
  measurement below argues is that a comparison taking each harness's defaults ranks tool
  inventories rather than harnesses, and this one is the extreme case: the same harness is
  both the most and the least expensive row.

- 2026-08-19 — **a harness's fixed cost is measurable without running the model, and
  Claude Code's default tool set is what makes it expensive** — 18,388 tokens of a 32,768
  context before the user's first word, against 3,711 at `--tools Bash,Edit,Read,Write`.
  The documented `CLAUDE_CODE_DISABLE_*` variables move that by 7% and remove no tool.
  All four harnesses in `harness/README.md`, rows in `docs/data/`.

- 2026-08-19 — **"no proxy is needed" holds, but the model's own chat template had to
  go.** Claude Code sends a `role: "system"` message *after* the user turn — the
  `mid-conversation-system-2026-04-07` capability — on every request, with 21 tools
  defined or with none, and `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1` does not stop
  it. Qwen3.8's template raises on a non-leading system message, llama.cpp returns
  that as a 500, and Claude Code retries ten times and dies. Anthropic documents an
  automatic retry that disables the capability after such a rejection, but it matches
  on the upstream's error wording, which a Jinja exception does not carry.
  `config/templates/qwen3.8-system-anywhere.jinja` differs from the shipped template in
  one line and the flow works, with nothing in the request path.

- 2026-08-19 — **a box is inserted before the terminal run: half the winning config
  could not reach the model.** Sampling and the thinking toggle are per-request
  everywhere else here, and Claude Code sends its own body with neither, so through
  `/v1/messages` this model serves at its `xhigh` default. `config/agent.env` and five
  optional flags in `scripts/serve.sh` make both server defaults.
- 2026-08-19 — the endpoint moves to port **8081**; 8080 was already held on the machine
  this is developed on.

- 2026-08-18 — `needs:` moves from 0004 to 0005. 0004 is dropped, so no sweep will name a
  winning quant; the config this feature wires in is Q4_K_M by elimination, and what it still
  waits on is the sampling and reasoning level 0005 settles. **This feature now also owns
  naming the context and KV cache type**, which 0004 would have swept — 0003 and 0014 bound
  them already (ingest time binds, not memory; 57,344 is the attended ceiling) but nothing has
  yet written down which to use.

- 2026-08-17 — retargeted from Cursor's built-in assistant to the Claude-Code-in-editor
  flow already in use. The earlier tunnel design is kept as a rejected alternative: the
  extension runs the agent locally, so the vendor backend drops out of the path entirely.
- 2026-08-17 — dropped `needs: 0010` and `review: human`. The privacy trade that required
  a human call belonged to the tunnel, which is no longer the path.
- 2026-08-17 — the open question about running network-off is answered from Anthropic's
  documented network requirements: it cannot. OAuth refresh and feature-flag fetches reach
  Anthropic regardless of `ANTHROPIC_BASE_URL`. The feature now scopes itself to local
  inference and hands the offline claim to 0010.
