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

echo "serving $CONFIG: ctx=$CTX_SIZE kv=$CACHE_TYPE_K/$CACHE_TYPE_V on $HOST:$PORT" >&2
exec llama-server "${args[@]}"
