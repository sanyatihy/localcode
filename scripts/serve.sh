#!/usr/bin/env bash
# Start llama-server from a config file and nothing else. The config is the single
# source of truth for how a measurement was produced; this script adds no flags.
set -euo pipefail

CONFIG="${1:-config/baseline.env}"
[ -f "$CONFIG" ] || { echo "no such config: $CONFIG" >&2; exit 2; }

# shellcheck source=/dev/null
set -a; . "$CONFIG"; set +a

for var in MODEL_HF HOST PORT CTX_SIZE CACHE_TYPE_K CACHE_TYPE_V FLASH_ATTN N_GPU_LAYERS PARALLEL; do
  [ -n "${!var:-}" ] || { echo "$CONFIG is missing $var" >&2; exit 2; }
done

args=(
  -hf "$MODEL_HF"
  --host "$HOST" --port "$PORT"
  --ctx-size "$CTX_SIZE"
  --cache-type-k "$CACHE_TYPE_K" --cache-type-v "$CACHE_TYPE_V"
  --flash-attn "$FLASH_ATTN"
  --n-gpu-layers "$N_GPU_LAYERS"
  --parallel "$PARALLEL"
)
[ "${JINJA:-0}" = "1" ] && args+=(--jinja)
# Prometheus counters at /metrics, off by default in llama-server. A harness builds its own
# requests, so what a run cost is only countable at the server: 0010 reads the token
# counters either side of a run and the difference is that run's.
[ "${METRICS:-0}" = "1" ] && args+=(--metrics)

# Optional serving defaults, absent from every config the scorer drives. The scorer
# sends sampling and the thinking toggle on each request; an editor agent sends its
# own request body and neither of those, so for that flow they have to be served as
# defaults or the config that runs is not the one that was chosen.
add_opt() { [ -n "${2:-}" ] && args+=("$1" "$2"); return 0; }
add_opt --temp "${TEMP:-}"
add_opt --top-p "${TOP_P:-}"
add_opt --top-k "${TOP_K:-}"
add_opt --presence-penalty "${PRESENCE_PENALTY:-}"
add_opt --chat-template-kwargs "${CHAT_TEMPLATE_KWARGS:-}"

# A template override is a path, and llama-server resolves it against its own working
# directory. Make it absolute here and fail on a missing file, rather than letting the
# server fall back to the model's own template and serve something nobody asked for.
if [ -n "${CHAT_TEMPLATE_FILE:-}" ]; then
  case "$CHAT_TEMPLATE_FILE" in /*) ;; *) CHAT_TEMPLATE_FILE="$PWD/$CHAT_TEMPLATE_FILE" ;; esac
  [ -f "$CHAT_TEMPLATE_FILE" ] || { echo "$CONFIG names a chat template that is not there: $CHAT_TEMPLATE_FILE" >&2; exit 2; }
  args+=(--chat-template-file "$CHAT_TEMPLATE_FILE")
fi

echo "serving $CONFIG: ctx=$CTX_SIZE kv=$CACHE_TYPE_K/$CACHE_TYPE_V on $HOST:$PORT" >&2
exec llama-server "${args[@]}"
