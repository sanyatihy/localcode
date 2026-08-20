#!/usr/bin/env bash
# Walk one serving config through both conditions and record a row per request: the
# conversation alone, then the same conversation with a small call between its turns.
#
# The restart between conditions is part of the measurement rather than tidiness. The
# server keeps prefixes it has evicted in host RAM, so a second condition replaying the
# same conversation would be served from the first one's leftovers and report a cache no
# session ever had. Each condition starts from a server that has seen nothing.
#
# The server's own log is kept per condition, because how it chose a slot is the
# explanation for whatever the rows say, and it is not recoverable afterwards.
set -euo pipefail

CONFIG="${1:-config/agent.env}"
[ -f "$CONFIG" ] || { echo "no such config: $CONFIG" >&2; exit 2; }
LABEL="${LABEL:-$(basename "$CONFIG" .env)}"
OUT="${OUT:-results/0018-prefix.jsonl}"
PROBE_ARGS="${PROBE_ARGS:-}"
# Which conditions to walk, in order. A re-walk usually asks about one of them, and each
# costs a server load and every turn's ingest.
CONDITIONS="${CONDITIONS:-clean interleaved}"

# shellcheck source=/dev/null
. "$CONFIG"
ENDPOINT="http://${HOST:-127.0.0.1}:${PORT:-8081}"
mkdir -p "$(dirname "$OUT")"

# An 18 GB process does not exit on a fixed sleep, and the next server binding while the
# old one still holds the port measures the previous config under the next one's name.
stop_server() {
  pkill -f llama-server 2>/dev/null || true
  local deadline=$((SECONDS + 90))
  while pgrep -f llama-server >/dev/null; do
    [ $SECONDS -ge $deadline ] && { pkill -9 -f llama-server 2>/dev/null || true; sleep 3; break; }
    sleep 1
  done
  sleep 2
}

# serve.sh validates its config before it execs, so for the first instants after launch
# there is no llama-server to find: the died-early shortcut needs a grace period or it
# fires on a server that goes on to load perfectly.
wait_healthy() {
  local deadline=$((SECONDS + 300)) grace=$((SECONDS + 20))
  while [ $SECONDS -lt $deadline ]; do
    curl -s -m 2 "$ENDPOINT/health" 2>/dev/null | grep -q '"ok"' && return 0
    if [ $SECONDS -ge $grace ] && ! pgrep -f llama-server >/dev/null; then return 1; fi
    sleep 3
  done
  return 1
}

# shellcheck disable=SC2086  # CONDITIONS is a list of conditions and must expand
for condition in $CONDITIONS; do
  echo "=== $LABEL: $condition ===" >&2
  stop_server
  log="results/serve-$LABEL-$condition.log"
  nohup ./scripts/serve.sh "$CONFIG" >"$log" 2>&1 &
  wait_healthy || { echo "  server did not come up; see $log" >&2; exit 2; }

  case "$condition" in
    clean)              flag="" ;;
    interleaved)        flag="-interleave" ;;
    interleaved-shared) flag="-interleave -small-shares-system" ;;
    *) echo "unknown condition: $condition" >&2; exit 2 ;;
  esac
  # shellcheck disable=SC2086
  go run ./cmd/prefixprobe -endpoint "$ENDPOINT" -config "$LABEL" -results "$OUT" $flag $PROBE_ARGS
done
stop_server
echo "=== rows in $OUT ===" >&2
