#!/usr/bin/env bash
# Shared by every script that starts and stops a server. Sourced, never executed, and so it
# sets no shell options: `set -euo pipefail` here would leak into whatever sourced it. Every
# caller sets them itself, and `make check` holds both halves of that to the file mode.
#
# These four functions were copied into four scripts and had already drifted into four
# different answers to the same two traps, which is exactly the failure a second home for a
# fact produces. Both traps are in docs/TECH.md, and neither may be re-solved locally.

# ENDPOINT and PROC are what a caller overrides; the defaults are this project's server.
ENDPOINT="${ENDPOINT:-http://127.0.0.1:8081}"
PROC="${PROC:-llama-server}"

# HEALTH_GRACE is the trap. A serve script validates its config and only then execs the
# server, so for the first instants after launch there is nothing for pgrep to find. Without
# the grace the first poll — curl refused in a millisecond, pgrep finding nothing — reports
# "load failed" for a server that goes on to load in 15 s. It fires only on a loaded machine,
# where the child is slow to be scheduled, which is when the measurement matters.
HEALTH_GRACE="${HEALTH_GRACE:-20}"

# server_alive reports whether the server this run started is still there. Overridable,
# because a runtime that sets its own process title cannot be found by pattern — MTPLX's
# command line is just the Python binary — and is tracked by pid instead.
server_alive() { pgrep -f "$PROC" >/dev/null; }

# SERVE_DAEMON is the node's serving LaunchDaemon (0056). On any other machine it is loaded
# nowhere, and serve_daemon_loaded answers no.
SERVE_DAEMON="${SERVE_DAEMON:-com.localcode.serve}"
serve_daemon_loaded() { launchctl print "system/$SERVE_DAEMON" >/dev/null 2>&1; }

# SERVE_SWITCH is the file that daemon is gated on (0061). launchd's KeepAlive PathState
# holds the server up while it exists and leaves it down while it does not, so starting and
# stopping are a file the serving user owns rather than a bootout needing root. The same path
# is written into the plist, where scripts/node/install.sh substitutes this user's home.
SERVE_SWITCH="${SERVE_SWITCH:-$HOME/.local/state/localcode/serve.on}"

# Which route stops that daemon, read from the plist that is installed rather than from the
# switch itself: a switched node with the server already stopped has no switch either, and
# a stop that asked for root there would refuse the second time it was run.
SERVE_PLIST="${SERVE_PLIST:-/Library/LaunchDaemons/$SERVE_DAEMON.plist}"
serve_daemon_switched() { grep -qF "$SERVE_SWITCH" "$SERVE_PLIST" 2>/dev/null; }

# stop_server ends it and waits for the memory back. The trap: an 18 GB process does not
# exit on a fixed sleep, and the next server binding while the old one still holds the port
# measures the previous config under the next config's name.
#
# STOP_CMD gives a runtime with a stop command of its own the chance to use it first.
stop_server() {
  # On the node the server is a LaunchDaemon with KeepAlive, so a failed server is replaced
  # and a stop has to end the service rather than the process. A switched daemon (0061) is
  # ended by removing the file it is gated on, which the serving user owns; a plist that
  # predates the switch still needs the bootout, and that needs root. When the bootout fails
  # there is nothing useful left to do, and going on to pkill would report a stop that did
  # not happen.
  if serve_daemon_loaded; then
    if serve_daemon_switched; then
      rm -f "$SERVE_SWITCH"
    elif ! launchctl bootout "system/$SERVE_DAEMON" >/dev/null 2>&1; then
      echo "stop_server: $SERVE_DAEMON is loaded and booting it out failed — run this under sudo" >&2
      return 1
    fi
  fi
  if [ -n "${STOP_CMD:-}" ]; then eval "$STOP_CMD" >/dev/null 2>&1 || true; fi
  pkill -f "$PROC" 2>/dev/null || true
  local deadline=$((SECONDS + 90))
  while pgrep -f "$PROC" >/dev/null; do
    [ $SECONDS -ge $deadline ] && { pkill -9 -f "$PROC" 2>/dev/null || true; sleep 3; break; }
    sleep 1
  done
  sleep 2
}

# wait_healthy <seconds>: 0 once the endpoint answers, 1 if it died or the wait ran out.
# llama-server answers 503 while it loads and 200 when it can serve, so the code is the
# readiness signal and the body is not read.
wait_healthy() {
  local deadline=$((SECONDS + $1)) grace=$((SECONDS + HEALTH_GRACE))
  while [ $SECONDS -lt $deadline ]; do
    [ "$(curl -s -m 2 -o /dev/null -w '%{http_code}' "$ENDPOINT/health" 2>/dev/null)" = "200" ] && return 0
    if [ $SECONDS -ge $grace ] && ! server_alive; then return 1; fi
    sleep 3
  done
  return 1
}

# reserve_gb prints what the system side keeps whatever the GPU is allowed, read from the
# machine file MACHINE names. RESERVE_GB in the environment still wins, because the reserve is
# a judgement about one machine's desk rather than a reading. Here rather than in each caller:
# gpuraise.sh refuses a raise that goes under it and rungs.sh holds it back from the ladder,
# and two answers to one number is how a node gets laddered on a laptop's desk. An integer —
# both callers do shell arithmetic with it.
reserve_gb() {
  if [ -n "${RESERVE_GB:-}" ]; then printf '%s' "$RESERVE_GB"; return 0; fi
  python3 -c '
import json, sys
try:
    print(json.load(open(sys.argv[1]))["reserve_gb"])
except (OSError, ValueError, KeyError) as e:
    sys.exit(f"{sys.argv[1]}: no reserve_gb to read ({e})")' "${MACHINE:-config/machine.json}"
}

# served_ctx asks the endpoint what it is serving, and prints `null` for a backend that does
# not say. The guard against measuring one config under another's name, so a caller that
# skips it is claiming rather than checking.
served_ctx() {
  curl -s -m 5 "$ENDPOINT/props" 2>/dev/null \
    | python3 -c "import sys,json;print(json.load(sys.stdin)['default_generation_settings']['n_ctx'])" 2>/dev/null \
    || echo null
}

# The readings a script takes off the machine itself differ by platform, and each has one
# home here: a caller that branches for itself is a second answer to the same question,
# which is the drift this file exists to prevent. Darwin is the laptop and the node; Linux
# is the GB10, which is headless and runs no compositor at all.
darwin() { [ "$(uname -s)" = "Darwin" ]; }

# total_memory_gb prints installed memory in whole GiB.
total_memory_gb() {
  if darwin; then
    echo $(( $(sysctl -n hw.memsize) / 1073741824 ))
  else
    awk '/^MemTotal:/ {printf "%d\n", $2 / 1048576}' /proc/meminfo
  fi
}

# weights_bytes prints the size of a weights file, following the symlink. The HuggingFace
# cache stores snapshots as links into blobs/, and the link's own size reads as a 0 GB model
# that silently inflates every headroom estimate built on it.
weights_bytes() { # path
  if darwin; then stat -L -f%z "$1"; else stat -L -c%s "$1"; fi
}

# memprobe_json prints one JSON object of the memory facts a cell is judged on, in the shape
# every row already carries. macOS reads it from scripts/memprobe.sh; Linux builds the same
# keys from /proc/meminfo, since pagesize, vm_stat and the Metal cap do not exist there and a
# script that shells out to them dies mid-cell under `set -e`.
#
# The figures Linux has no counterpart for are `null`, never 0: the compressor is macOS's,
# and wired memory and the GPU wired cap are Metal's — a zero there would read as a machine
# holding nothing against a ceiling of nothing. What replaces them on the GB10 is a
# measurement that box has not been reached for; `available_gb` is the reading the Linux
# headroom rule uses in the meantime.
memprobe_json() {
  if darwin; then ./scripts/memprobe.sh; return 0; fi
  local rss
  rss=$(ps -Ao rss,comm | awk '/llama-server/ {s+=$1} END {printf "%.3f", s/1048576}')
  RSS="${rss:-0}" python3 -c '
import json, os
kb = {}
with open("/proc/meminfo") as fh:
    for line in fh:
        name, _, rest = line.partition(":")
        parts = rest.split()
        if parts:
            try:
                kb[name] = float(parts[0])
            except ValueError:
                pass
print(json.dumps({
    "free_gb": round(kb["MemFree"] / 1048576, 3),
    "available_gb": round(kb["MemAvailable"] / 1048576, 3),
    "compressed_gb": None,
    "swap_used_mb": round((kb["SwapTotal"] - kb["SwapFree"]) / 1024, 3),
    "llama_rss_gb": float(os.environ["RSS"] or 0),
    "anonymous_gb": round(kb["AnonPages"] / 1048576, 3),
    "wired_gb": None,
    "wired_limit_mb": None,
    "wired_limit_gb": None,
    "wired_limit_source": "not_applicable",
    "wired_headroom_gb": None,
}))'
}

# apparatus_anonymous_gb prints the memory in use by everything that is not the model, in
# the one unit that competes with it. A sum of per-process RSS counts every shared page once
# per resident process and overstated a measured state by about 1.5x, which is why this is
# anonymous memory and not that.
apparatus_anonymous_gb() {
  memprobe_json | python3 -c "import sys,json;print(json.load(sys.stdin)['anonymous_gb'])"
}

# served_slots prints how many slots the endpoint serves, and `null` for a backend that does
# not say. One slot serves one developer and the GB10 serves four; a ladder that fills one of
# four has measured a config nobody runs.
served_slots() {
  curl -s -m 5 "$ENDPOINT/props" 2>/dev/null \
    | python3 -c "import sys,json;print(json.load(sys.stdin)['total_slots'])" 2>/dev/null \
    || echo null
}

# per_slot_ctx prints the context each slot is serving, given the context that was asked for.
# `null` when the endpoint did not say, and `ambiguous` when what came back resolves to
# neither shape — on which a caller measures nothing.
#
# Two shapes exist and the response alone cannot tell them apart: `n_ctx` is the slot's share
# on one build and the whole server's on another, and both sit beside the same `total_slots`.
# The request is what breaks the tie — a reply equal to the asked-for context over the slots
# is the per-slot shape, one equal to the asked-for context is the total — and both resolve
# to the same per-slot number, which is what the fill has to be sized against.
per_slot_ctx() { # requested_ctx
  local want="$1" got slots
  got=$(served_ctx)
  slots=$(served_slots)
  [ "$got" = null ] && { echo null; return 0; }
  [ "$slots" = null ] && slots=1
  if [ "$slots" -le 1 ]; then echo "$got"; return 0; fi
  if [ $(( want % slots )) -ne 0 ]; then echo ambiguous; return 0; fi
  if [ "$got" -eq $(( want / slots )) ]; then echo "$got"; return 0; fi
  if [ "$got" -eq "$want" ]; then echo $(( want / slots )); return 0; fi
  echo ambiguous
}

# desk_baseline prints the compositor's rate in cores before any model is loaded. Per run,
# because the machine's baseline is not a constant and a desktop verdict has to carry the
# basis it was judged against.
#
# macOS only: a headless box has no compositor to lose, so there is no baseline to take and
# `not_applicable` is the honest answer — the same one an unattended cell's verdict carries.
desk_baseline() {
  if ! darwin; then echo not_applicable; return 0; fi
  local a b
  a=$(./scripts/deskprobe.sh); sleep 6; b=$(./scripts/deskprobe.sh)
  A="$a" B="$b" python3 -c "
import json, os
a, b = json.loads(os.environ['A']), json.loads(os.environ['B'])
span = b['t'] - a['t']
print(round((b['windowserver_cpu_seconds'] - a['windowserver_cpu_seconds']) / span, 3) if span > 0 else 0)"
}

# cell_config writes one ladder cell: a base serving config with the context and KV type
# this rung is asking about. Generated rather than committed, because a rung written by hand
# is a fact about one machine — the defect docs/VISION.md names first.
cell_config() { # base ctx kv out
  local base="$1" ctx="$2" kv="$3" out="$4"
  # The base's comments are dropped, not carried: they describe the base's context and KV
  # type, and a generated file explaining itself as something else is worse than one that
  # says nothing. Settings only, then this rung's two.
  {
    printf '# Generated by scripts/ladder.sh from %s. Not committed: a rung is derived.\n' "$base"
    grep -E '^[A-Z_]+=' "$base" | grep -vE '^(CTX_SIZE|CACHE_TYPE_K|CACHE_TYPE_V)='
    printf 'CTX_SIZE="%s"\nCACHE_TYPE_K="%s"\nCACHE_TYPE_V="%s"\n' "$ctx" "$kv" "$kv"
  } > "$out"
}

# batch_config writes one prefill-batch cell: the base's settings plus the physical batch
# this cell asks about. The logical ceiling follows only where it would otherwise clamp
# that value, so below it nothing but the physical batch moves. Generated for the same
# reason a rung is — two scripts walk these cells, and a second copy of the rule is a
# second answer to it.
batch_config() { # base ubatch out
  local base="$1" ubatch="$2" out="$3" batch=2048   # llama.cpp's own --batch-size default
  [ "$ubatch" -gt "$batch" ] && batch="$ubatch"
  {
    printf '# Generated from %s. Not committed: a cell is derived.\n' "$base"
    grep -E '^[A-Z_]+=' "$base"
    printf 'BATCH_SIZE="%s"\nUBATCH_SIZE="%s"\n' "$batch" "$ubatch"
  } > "$out"
}

# built echoes the path to a freshly built command, and builds it. Rows carry the revision
# the binary was built from, and `go run` records none — so anything that writes a results
# file goes through this rather than running from source.
built() { # cmd
  local out=".bin/$1"
  go build -o "$out" "./cmd/$1" >&2 || return 1
  printf '%s' "$out"
}
