#!/usr/bin/env bash
# Serve the MLX model on an OpenAI-compatible endpoint, so 0002's scorer drives it with
# no change beyond -endpoint. Mirrors scripts/serve.sh: the config file is the single
# source of truth for how a measurement was produced, and this script adds no flags.
#
# Port differs from llama.cpp's by default so both can be up at once — but they must not
# be *loaded* at once on 32 GB, where either alone is most of the machine.
set -euo pipefail
CONFIG="${1:-mlx/config/mlx-4bit.env}"
[ -f "$CONFIG" ] || { echo "no such config: $CONFIG" >&2; exit 2; }
cd "$(dirname "$0")/.."

# shellcheck source=/dev/null
set -a; . "$CONFIG"; set +a
for var in MODEL_HF HOST PORT MAX_TOKENS; do
  [ -n "${!var:-}" ] || { echo "$CONFIG is missing $var" >&2; exit 2; }
done

echo "serving $CONFIG: $MODEL_HF on $HOST:$PORT" >&2
exec ./mlx/.venv/bin/python -m mlx_lm server \
  --model "$MODEL_HF" --host "$HOST" --port "$PORT" --max-tokens "$MAX_TOKENS"
