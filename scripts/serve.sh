#!/usr/bin/env bash
# Start llama-server from a config file and nothing else. The config is the single
# source of truth for how a measurement was produced; this script adds no flags.
set -euo pipefail

CONFIG="${1:-config/tuned.env}"
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

# Prefill batching, absent from every config that leaves llama.cpp's defaults. --ubatch-size
# is the physical batch: it sizes the compute buffer and the Metal dispatch, and so it is the
# flag prefill time answers to. --batch-size is the logical ceiling on it. Both live in the
# config, because a number produced by a flag at the call site is attributable to no config.
add_opt --batch-size "${BATCH_SIZE:-}"
add_opt --ubatch-size "${UBATCH_SIZE:-}"

# The host-RAM prompt cache, absent from every config that leaves llama.cpp's 8192 MiB
# default. On unified memory those MiB are bought from the same pool the KV reservation
# sits in, so how many a measurement was taken with is part of how it was produced. `0`
# turns the cache off and is a value, not an absence — add_opt tests for empty, not false.
add_opt --cache-ram "${CACHE_RAM:-}"

# Speculative decoding, absent from every config that does not use it. The draft model and
# the mechanism are part of how a measurement was produced, so they live in the config with
# everything else rather than being passed at the call site.
add_opt --spec-draft-hf "${SPEC_DRAFT_HF:-}"
add_opt --spec-type "${SPEC_TYPE:-}"
add_opt --spec-draft-n-max "${SPEC_DRAFT_N_MAX:-}"

# A template override is a path, and llama-server resolves it against its own working
# directory. Make it absolute here and fail on a missing file, rather than letting the
# server fall back to the model's own template and serve something nobody asked for.
if [ -n "${CHAT_TEMPLATE_FILE:-}" ]; then
  case "$CHAT_TEMPLATE_FILE" in /*) ;; *) CHAT_TEMPLATE_FILE="$PWD/$CHAT_TEMPLATE_FILE" ;; esac
  [ -f "$CHAT_TEMPLATE_FILE" ] || { echo "$CONFIG names a chat template that is not there: $CHAT_TEMPLATE_FILE" >&2; exit 2; }
  args+=(--chat-template-file "$CHAT_TEMPLATE_FILE")
fi

# Which build served it is part of the record too: 0017 measures a mechanism that exists
# only in an unmerged pull request, so the binary that has it cannot be the one on PATH.
SERVER_BIN="${SERVER_BIN:-llama-server}"
command -v "$SERVER_BIN" >/dev/null 2>&1 || [ -x "$SERVER_BIN" ] || {
  echo "$CONFIG names a server that is not there: $SERVER_BIN" >&2; exit 2; }

echo "serving $CONFIG: ctx=$CTX_SIZE kv=$CACHE_TYPE_K/$CACHE_TYPE_V on $HOST:$PORT via $SERVER_BIN" >&2
exec "$SERVER_BIN" "${args[@]}"
