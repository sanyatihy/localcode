#!/usr/bin/env bash
# Serve Splash from a config file and nothing else. Mirrors scripts/serve.sh and
# runtimes/mlx/serve.sh: the config is the single source of truth for how a measurement was
# produced, and this script adds no flags.
set -euo pipefail
CONFIG="${1:-runtimes/splash/config/splash-27b.env}"
# Before the check, so a relative path means one file: checked where the caller stood and
# sourced from the checkout, it could pass as one config and load as another.
cd "$(dirname "$0")/../.."
[ -f "$CONFIG" ] || { echo "no such config, from the checkout's root: $CONFIG" >&2; exit 2; }

# shellcheck source=/dev/null
set -a; . "$CONFIG"; set +a
for var in MODEL_PACKAGE HOST PORT MAX_CONTEXT MAX_MEMORY; do
  [ -n "${!var:-}" ] || { echo "$CONFIG is missing $var" >&2; exit 2; }
done
# The launcher binds 127.0.0.1:8000 and takes no flag for either, so a config naming
# another address would be served nowhere while every row carried it. Refused here.
[ "$HOST:$PORT" = "127.0.0.1:8000" ] || {
  echo "$CONFIG serves $HOST:$PORT, and splash binds only 127.0.0.1:8000" >&2; exit 2; }

# The node's route to the Hugging Face CDN stalls, and Splash checks the Hub for the
# repository's main before every load. Offline it falls back to the staged snapshot, whose
# artifacts it verifies against the manifest either way, so for one snapshot this changes
# where the weights are found and not what is served; online, a moved main would be served — 0058's `LLAMA_ARG_OFFLINE=1` on the llama.cpp side.
# Overridable, because the first install on a machine has to reach the Hub once.
export HF_HUB_OFFLINE="${HF_HUB_OFFLINE:-1}"

SPLASH_BIN="${SPLASH_BIN:-splash}"
command -v "$SPLASH_BIN" >/dev/null 2>&1 || [ -x "$SPLASH_BIN" ] || {
  echo "no splash on PATH: run runtimes/splash/setup.sh, or set SPLASH_BIN" >&2; exit 2; }

# Tolerated: under pipefail a failing --version would end this script with no message, and
# the line below already says "unknown" for it.
VERSION=$("$SPLASH_BIN" --version 2>&1 | tr -d '\n') || VERSION=""
echo "serving $CONFIG: $MODEL_PACKAGE ctx=$MAX_CONTEXT max-memory=$MAX_MEMORY \
offline=$HF_HUB_OFFLINE version=${VERSION:-unknown} on $HOST:$PORT" >&2
exec "$SPLASH_BIN" serve \
  --model "$MODEL_PACKAGE" --max-context "$MAX_CONTEXT" --max-memory "$MAX_MEMORY"
