#!/usr/bin/env bash
# Score one runtime on the ranking suite plus the depth suite, under the config 0005
# settled. Run once per runtime, with only -endpoint differing — that is the whole point
# of 0012's generalisation, and any other difference between the two invocations makes
# the comparison measure the difference instead of the runtime.
#
#   ./mlx/compare.sh llamacpp http://127.0.0.1:8081
#   ./mlx/compare.sh mlx      http://127.0.0.1:8082
set -uo pipefail
cd "$(dirname "$0")/.."
LABEL="${1:?usage: compare.sh <label> <endpoint>}"
EP="${2:?usage: compare.sh <label> <endpoint>}"
RES="${RES:-results/0006-runtime.jsonl}"

# Thinking off at the non-thinking sampling pair: 0005's settled config, and the one
# setting both runtimes can be driven to identically. Effort is deliberately not sent —
# see the log entry of 2026-08-18: it travels top-level on llama.cpp and only inside
# chat_template_kwargs on MLX, so sending it would compare two reasoning levels.
for suite in tasks tasks/depth; do
  echo "=== $LABEL: $suite ==="
  go run ./cmd/eval -tasks "$suite" -n 3 -config "0006-$LABEL" \
     -thinking off -sampling-profile nonthinking \
     -results "$RES" -endpoint "$EP" || true
done
echo "=== $LABEL complete ==="
