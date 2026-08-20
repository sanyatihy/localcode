#!/usr/bin/env bash
# Serve candidate A on an OpenAI-compatible endpoint, so the scorer drives it with no change
# beyond -endpoint. Mirrors scripts/serve.sh and mlx/serve.sh: the config file is the single
# source of truth for how a measurement was produced, and this script adds no flags.
#
# Port differs from llama.cpp's and MLX's so all three can be configured at once — but they
# must not be *loaded* at once on 32 GB, where any one of them is most of the machine.
set -euo pipefail
CONFIG="${1:-mtplx/config/optimized-speed-fp16-32k.env}"
[ -f "$CONFIG" ] || { echo "no such config: $CONFIG" >&2; exit 2; }
cd "$(dirname "$0")/.."

# shellcheck source=/dev/null
set -a; . "$CONFIG"; set +a
for var in MODEL_HF HOST PORT PROFILE CONTEXT_WINDOW MAX_TOKENS GENERATION_MODE; do
  [ -n "${!var:-}" ] || { echo "$CONFIG is missing $var" >&2; exit 2; }
done

# --no-auth: the server otherwise mints an API key, and a harness that has to read one back
# out of a state directory is a harness that measures its own plumbing. Loopback only.
args=(
  --model "$MODEL_HF" --host "$HOST" --port "$PORT"
  --profile "$PROFILE" --context-window "$CONTEXT_WINDOW"
  --max-tokens "$MAX_TOKENS" --generation-mode "$GENERATION_MODE" --no-auth
)
# Depth is the draft length. Left unset the server auto-tunes it, which is what candidate A
# is asked to do here; set, it is a measurement of one depth and the row must say which.
[ -n "${DEPTH:-}" ] && args+=(--depth "$DEPTH")
# The KV cache is where this runtime spends its context, and quantising it is the same lever
# config/tuned.env already pulls for llama.cpp. Off is the server's default; a config that
# sets it is measuring a different configuration and says so.
[ -n "${PAGED_KV_QUANTIZATION:-}" ] && args+=(--paged-kv-quantization "$PAGED_KV_QUANTIZATION")

echo "serving $CONFIG: $MODEL_HF ctx=$CONTEXT_WINDOW profile=$PROFILE mode=$GENERATION_MODE on $HOST:$PORT" >&2
exec ./mtplx/.venv/bin/mtplx serve "${args[@]}"
