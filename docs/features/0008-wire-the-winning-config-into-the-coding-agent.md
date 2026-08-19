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

**This flow is offline under token authentication.** Reasoned from Anthropic's documented
network requirements it is not: OAuth refresh and feature-flag fetches are described as
reaching Anthropic whatever `ANTHROPIC_BASE_URL` says. Measured, a session doing a real
task with `ANTHROPIC_AUTH_TOKEN` set and `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`
contacts **no host at all**, and completes unchanged with every remote host unreachable.

The claim is scoped to token auth, and the scope is not a formality: without a credential
the session refuses to start, and the OAuth path is untested here because no claude.ai
login is stored on this machine. What the configuration costs is the feature-flag set —
auto mode as a starting permission mode, Remote Control, cross-session messaging.

## Tasks

- [x] Claude Code's background/small-model calls are characterised from its own documentation, and pointed at the local endpoint or disabled, with the exact variables committed
- [x] The half of the winning config that is per-request — thinking off and its sampling — is served as a default, since Claude Code sends neither
- [x] `ANTHROPIC_BASE_URL` against the local `/v1/messages` completes a real task in this repo from the terminal, with no proxy
- [x] Tool-call validity through the Anthropic→OpenAI conversion is measured and compared against 0005's numbers on the native path
- [x] Context exhaustion and auto-compaction at the measured ceiling are made visible rather than silent
- [x] The residual traffic is measured, not taken from the docs: what hosts the flow contacts during a real session, and which stop when non-essential traffic is disabled
- [x] The failure mode when Anthropic hosts are unreachable is recorded — whether the session degrades, blocks, or refuses to start — since that is what an outage or a flight actually looks like
- [ ] The same works driven from the Cursor/VSCode extension, matching the current flow, and the transcript is recorded
- [x] The setup is committed as configuration, and `docs/TECH.md` records it, the residual traffic, and the rejection of Cursor's built-in assistant with its reason

## Log
- 2026-08-19 — **the editor flow does not fit the scorer's context, so `config/agent.env`
  serves 49,152.** An extension session's first request measured 36,309 tokens — the full
  tool set, the project's instructions and the editor's own context — against a 32,768
  server, and took llama-server's 400 before anything was typed. The terminal avoids this
  with `--tools`, which the extension has no equivalent of. That turn then cost **7.5
  minutes**: 434 s of ingest and 82 tokens at 5.54 tok/s against 9.8 on a short prompt, so
  depth taxes decode as well as prefill. Raising the context buys nothing: prefill costs
  what the prompt is, not what is reserved. It also corrects a claim below — the declared
  window catches an overflow **between turns**, not the preamble a session starts with.

- 2026-08-19 — **the design's offline paragraph is reversed by measurement**, and rewritten
  above. It said this flow cannot be made offline and handed the offline claim to 0010; as
  committed the session contacts no host, and with every remote CONNECT refused it still
  completes. Unset `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` and the same task makes 9
  connections to `api.anthropic.com` and reaches no other host. Refusing to start is a
  missing credential, not an unreachable one.
- 2026-08-19 — **the context ceiling is declared rather than discovered.** Undeclared, the
  overflowing request goes out and llama-server's 400 comes back verbatim, unrecovered;
  declared, the conversation is counted through `count_tokens` against the served tokeniser
  and refused before ingest. The documented variable for an unrecognised id waits for
  Anthropic's too-long error, which this server never sends — the same wording mismatch
  that killed the mid-conversation-system retry, now twice.
- 2026-08-19 — **the Anthropic→OpenAI conversion costs nothing measurable**: the tool-call
  fixtures down `/v1/messages` are identical to 0005's native-path rows cell for cell.
  `cmd/eval -api messages` sends the other dialect to the same grader, which is what makes
  them comparable; it is not the backend seam 0012 owns.
- 2026-08-19 — **Claude Code becomes one harness among four rather than the destination.**
  Its configuration moves to `harness/claude-code/`, `internal/harness` gains an adapter so
  `cmd/tier2` drives it on the same terms, and it passes `patch-nil-check` against the
  unseen test. Measuring all four first: its fixed preamble is the largest of the four on
  its defaults and the smallest under `--tools`, so a comparison taking defaults would rank
  tool inventories rather than harnesses. The editor box moves below the boxes that need
  nobody at the keyboard.
- 2026-08-19 — **"no proxy is needed" holds, but the model's own chat template had to go.**
  Claude Code sends a `role: "system"` message after the user turn on every request, with
  21 tools defined or with none, and `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1` does not
  stop it; Qwen3.8's template raises on it and llama.cpp returns a 500.
  `config/templates/qwen3.8-system-anywhere.jinja` differs in one line.
- 2026-08-19 — **a box is inserted before the terminal run: half the winning config could
  not reach the model.** Sampling and the thinking toggle are per-request everywhere else
  here and Claude Code sends neither, so this model served at its `xhigh` default.
  `config/agent.env` and five optional flags in `scripts/serve.sh` make them defaults.
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
