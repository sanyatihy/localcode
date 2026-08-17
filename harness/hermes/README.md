# Hermes — blocked

Hermes Agent is configured and **recognised**, but cannot reach a loopback endpoint.
Recorded here so the next attempt starts from what was established rather than repeating it.

## The configuration, which is correct as far as it goes

`hermes config set` writes `~/.hermes/config.yaml`. There is no project-local config, so
this is global state on the machine — unlike Pi and OpenCode, whose settings live in this
repo.

    hermes config set providers.local.base_url     http://127.0.0.1:8080/v1
    hermes config set providers.local.api          openai
    hermes config set providers.local.api_mode     chat
    hermes config set providers.local.key_env      LOCAL_OPENAI_API_KEY
    hermes config set providers.local.default_model bartowski/Qwen3.8-27B-GGUF:Q4_K_M

## What is established

**Hermes reads the provider.** Proven by control: `--provider does-not-exist-xyz` fails
with *"Unknown provider"*, while `--provider local` fails with *"Connection error"*. It
resolves our entry and then cannot connect.

**The endpoint is reachable from the same shell**, at the same moment: `curl` to
`/v1/models` returns HTTP 200, and both Pi and OpenCode complete the same task against it.

**llama-server logs no incoming request**, so the failure is before the wire.

## Ruled out

- Egress firewall — `hermes egress status` reports it disabled, binary missing, not listening.
- Proxy environment — no `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY` set.
- The `/v1` suffix — fails identically with and without it.
- `api` versus `api_mode` — fails with either.

## Not ruled out

- A sandbox around the runtime. Hermes advertises "sandboxed code execution via Unix
  socket RPC", and a network-isolated sandbox would make `127.0.0.1` inside it a different
  host from the one serving the model. This is the leading hypothesis and the cheapest
  next test: bind the server to a LAN address and point Hermes at that. It is not tried
  here because the vision binds serving to loopback, so it needs a deliberate exception.
- `hermes config set` storing a JSON list as a string — `providers.local.models` came back
  as `'["..."]'`. Unrelated to the connection failure, but it means list-valued keys have
  to be written into the YAML by hand.

## Consequence

0010's comparison runs with the harnesses that work. Hermes is excluded on a *transport*
problem rather than on any quality result, and that distinction has to survive into the
write-up: "not measured" is not "worse".
