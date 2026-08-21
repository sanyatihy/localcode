# Scripts

The shell around a measurement: starting a server, walking a sweep, reading the machine.
Anything that has to be reasoned about is Go, under [`cmd/`](../cmd); these are the parts
that are genuinely shell — process lifecycle, `sysctl`, `vm_stat`, `curl`.

All run under `bash` via shebang, from the repo root, and write their rows to
`results/*.jsonl`.

| script | what it does |
|---|---|
| [`lib.sh`](lib.sh) | sourced by every script below that starts a server: stopping it, waiting for it, and the traps neither may re-solve |
| [`serve.sh`](serve.sh) | start `llama-server` from a config file, adding no flags of its own |
| [`smoke.sh`](smoke.sh) | assert a running endpoint answers and round-trips a tool call — `make smoke` |
| [`rungs.sh`](rungs.sh) | derive which contexts to ladder over from this machine, and report which of memory or time binds |
| [`ladder.sh`](ladder.sh) | walk each **derived** rung to a genuinely full context and record what it cost |
| [`screen.sh`](screen.sh) | judge one config admissible or not for the price of a load, instead of a full ladder cell |
| [`pair.sh`](pair.sh) | run a candidate and its baseline back to back on one machine state, then report the ratio |
| [`prefixrun.sh`](prefixrun.sh) | walk one config through the prefix-cache conditions, restarting the server between them |
| [`memprobe.sh`](memprobe.sh) | one JSON object of the memory facts that decide whether a config is viable |
| [`deskprobe.sh`](deskprobe.sh) | one JSON object of the compositor's state, so the desktop is judged from outside the model process |
| [`deskverdict.py`](deskverdict.py) | the desktop rule itself, in one place because `ladder.sh` and `screen.sh` both apply it |
| [`doclinks.py`](doclinks.py) | every relative link and heading anchor in tracked markdown resolves — `make docs` |
| [`claude-code-settings.sh`](claude-code-settings.sh) | generate the project-scoped settings file the editor extension reads |

Two of them take the machine's own answer rather than a list somebody typed:

    ./scripts/ladder.sh                                  # every rung rungs.sh derives, at the base KV type
    CELLS="40960:q8_0 57344:q8_0" ./scripts/ladder.sh     # one band, when that is the question
    BASE=config/agent.env ./scripts/ladder.sh             # ladder a different serving config

## Rules with one home, and no second

- **Killing an 18 GB server is not instant.** A fixed `sleep` lets the next server fail to
  bind while the health check passes against the *old* one, silently measuring the previous
  config under the next config's name.
- **A health check must not conclude "dead" before the process exists.** `serve.sh` validates
  its config and only then `exec`s, so the first poll finds nothing.
- Both live in [`lib.sh`](lib.sh), and the desktop verdict in
  [`deskverdict.py`](deskverdict.py). They were copied into four scripts once and had already
  drifted into four different answers to the same two questions, which is the failure a second
  home for a rule produces. Neither may be re-solved locally.

The reasons behind each are in [`docs/TECH.md`](../docs/TECH.md#gotchas); each has already
cost this repo a wrong number.
