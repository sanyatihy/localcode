# Scripts

The shell around a measurement: starting a server, walking a sweep, reading the machine.
Anything that has to be reasoned about is Go, under [`cmd/`](../cmd); these are the parts
that are genuinely shell — process lifecycle, `sysctl`, `vm_stat`, `curl`.

All run under `bash` via shebang, from the repo root, and write their rows to
`results/*.jsonl`.

| script | what it does |
|---|---|
| [`serve.sh`](serve.sh) | start `llama-server` from a config file, adding no flags of its own |
| [`smoke.sh`](smoke.sh) | assert a running endpoint answers and round-trips a tool call — `make smoke` |
| [`rungs.sh`](rungs.sh) | derive which contexts to ladder over from this machine, and report which of memory or time binds |
| [`ladder.sh`](ladder.sh) | walk each ladder cell to a genuinely full context and record what it cost |
| [`screen.sh`](screen.sh) | judge one config admissible or not for the price of a load, instead of a full ladder cell |
| [`pair.sh`](pair.sh) | run a candidate and its baseline back to back on one machine state, then report the ratio |
| [`prefixrun.sh`](prefixrun.sh) | walk one config through the prefix-cache conditions, restarting the server between them |
| [`matrix-request-level.sh`](matrix-request-level.sh) | the toggle matrix that needs no server restart |
| [`memprobe.sh`](memprobe.sh) | one JSON object of the memory facts that decide whether a config is viable |
| [`deskprobe.sh`](deskprobe.sh) | one JSON object of the compositor's state, so the desktop is judged from outside the model process |
| [`deskverdict.py`](deskverdict.py) | the desktop rule itself, in one place because `ladder.sh` and `screen.sh` both apply it |
| [`claude-code-settings.sh`](claude-code-settings.sh) | generate the project-scoped settings file the editor extension reads |

## Three things they all get right, and none of them may stop getting right

- **Killing an 18 GB server is not instant.** Every script that restarts one polls until
  the process is gone. A fixed `sleep` lets the next server fail to bind while the health
  check passes against the *old* one — which measures the previous config under the next
  config's name.
- **A health check must not conclude "dead" before the process exists.** `serve.sh`
  validates its config and only then `exec`s, so the first poll finds nothing. Every wait
  loop has a grace period before the died-early shortcut can fire.
- **The desktop verdict has one home.** `deskverdict.py`, imported by both instruments. A
  threshold with two homes drifts, and the verdict is the whole point of both.

The reasons behind each are in [`docs/TECH.md`](../docs/TECH.md#gotchas); each has already
cost this repo a wrong number.
