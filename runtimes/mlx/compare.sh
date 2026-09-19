#!/usr/bin/env bash
# Score one runtime on the ranking suite plus the depth suite, under the config 0005
# settled. Run once per runtime, with only -endpoint differing — that is the whole point
# of 0012's generalisation, and any other difference between the two invocations makes
# the comparison measure the difference instead of the runtime.
#
#   ./runtimes/mlx/compare.sh llamacpp http://127.0.0.1:8081
#   ./runtimes/mlx/compare.sh mlx      http://127.0.0.1:8082
set -euo pipefail
cd "$(dirname "$0")/../.." || exit 2
LABEL="${1:?usage: compare.sh <label> <endpoint>}"
EP="${2:?usage: compare.sh <label> <endpoint>}"
# The feature a comparison belongs to labels its rows. 0006 by default, which is what this
# script was written for; 0058 and 0060 re-ran it for other runtimes on other machines.
PREFIX="${PREFIX:-0006}"
RES="${RES:-results/$PREFIX-runtime.jsonl}"
# Scorer flags a runtime forces on the comparison, given to every invocation of it: both
# sides have to be sent the same ones, or the difference is the flags (scripts/pair.sh).
EVAL_ARGS="${EVAL_ARGS:-}"

# Thinking off at the non-thinking sampling pair: 0005's settled config, and the one
# setting both runtimes can be driven to identically. Effort is deliberately not sent —
# see the log entry of 2026-08-18: it travels top-level on llama.cpp and only inside
# chat_template_kwargs on MLX, so sending it would compare two reasoning levels.
for suite in tasks tasks/depth; do
  echo "=== $LABEL: $suite ==="
  # shellcheck disable=SC2086 # EVAL_ARGS is a flag list, split on purpose
  go run ./cmd/eval -tasks "$suite" -n 3 -config "$PREFIX-$LABEL" \
     -thinking off -sampling-profile nonthinking \
     -results "$RES" -endpoint "$EP" $EVAL_ARGS || true
done
echo "=== $LABEL complete ==="
