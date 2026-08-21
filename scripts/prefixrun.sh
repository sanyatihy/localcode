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
cd "$(dirname "$0")/.."
# shellcheck source=scripts/lib.sh
. scripts/lib.sh

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
ENDPOINT="http://${HOST:-127.0.0.1}:${PORT:-8081}"  # from the config just sourced
mkdir -p "$(dirname "$OUT")"

# shellcheck disable=SC2086  # CONDITIONS is a list of conditions and must expand
for condition in $CONDITIONS; do
  echo "=== $LABEL: $condition ===" >&2
  stop_server
  log="results/serve-$LABEL-$condition.log"
  nohup ./scripts/serve.sh "$CONFIG" >"$log" 2>&1 &
  wait_healthy 300 || { echo "  server did not come up; see $log" >&2; exit 2; }

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
