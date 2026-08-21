---
id: 0024
title: Give a config file exactly one reading
status: Draft        # Draft | Shipped | Dropped — kit ship and kit drop write it
created: 2026-08-21
shipped:             # written by kit ship, never by hand
needs:               # feature ids that must ship first, e.g. 0002, 0003. Empty = can start now
---

## Problem

A config file is the record of how a measurement was produced and `scripts/serve.sh` "adds
no flags of its own", but it sources the file with `set -a` and then reads the same names
from whatever the calling shell holds: `TEMP=0.5 ./scripts/serve.sh config/tuned.env`
launches with `--temp 0.5` against a config that deliberately sets no sampling. The banner
prints four fields rather than the argument vector and `Row` records no served sampling, so
the extra flag reaches neither the log nor the data. The same file format has a second
reader in Go, and `parseEnvFile` takes `KEY="v" # note` as the four-word value a shell would
never produce.

## Non-goals

- **Not a config schema.** Values stay unvalidated against the server; `serve.sh` already
  refuses a missing required name and a template path that is not there, which is the check
  that has caught something.
- **Not removing overrides.** A variant is still a new file. What goes is the *silent*
  override — one nothing records.
- **Not merging the two readers.** The shell sources these files and Go parses one of them,
  and both are the right tool where they sit; they must agree, not become one.
- **Not `env -i`.** The server needs `PATH` and `HOME`, and a whitelist of everything
  llama.cpp reads is a list that rots.

## Design

**Only the file decides.** `serve.sh` unsets every optional name before sourcing, so an
inherited `TEMP`, `SPEC_TYPE` or `CHAT_TEMPLATE_FILE` cannot survive into the argument
vector. The repo already does exactly this one directory over: `EnvFromFile` strips every
inherited `ANTHROPIC_*` and `CLAUDE_*` because "a stray `ANTHROPIC_API_KEY` outranks the
file's credential".

**The launch line is the argument vector.** The banner prints what is about to be exec'd.
Prevention that leaves no trace is untestable after the fact, and a server log is the only
record for traffic nobody scripted.

**The row records what was served, not only what was requested.** `Client.Props` already
decodes `default_generation_settings` for `n_ctx`; temperature, top_p, top_k and
presence_penalty sit in the same object and go onto `Row`. This is the half that survives a
mistake: a run served by a config that set sampling becomes visible in the results file
rather than only preventable at launch.

**The Go parser refuses what a shell would read differently.** An unquoted trailing comment
and a name that is not a shell identifier become errors, matching the comment that already
claims the parser is "strict on purpose".

**The editor's declared window is derived.** `CLAUDE_CODE_MAX_CONTEXT_TOKENS` is
`config/agent.env`'s `CTX_SIZE` less the output reservation, written out by hand in a second
file with nothing checking the arithmetic. A test that reads both files is enough; the
symptom it prevents is the `exceed_context_size_error` the comment exists to avoid.

## Tasks

- [ ] An inherited `TEMP`, `TOP_K` or `CHAT_TEMPLATE_FILE` cannot change what `scripts/serve.sh` launches
- [ ] The line `scripts/serve.sh` prints before `exec` is the whole argument vector
- [ ] Every results row records the sampling the server was serving, so a config that set some is detectable after the run
- [ ] `internal/harness`'s env parser refuses a line a shell would read differently, rather than accepting a value the shell never produces
- [ ] The editor's declared context window is checked against `config/agent.env` instead of being written twice
- [ ] `scripts/pair.sh` checks what the server is serving before it scores it, as `ladder.sh` and `screen.sh` do

## Open questions

None.

## Log
