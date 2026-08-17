#!/usr/bin/env bash
# Request-level toggle matrix: the arm of the sweep that needs no server restart.
#
# Thinking and sampling move TOGETHER. The model card gives different recommended
# values per mode, so sweeping the toggle at one fixed temperature would measure the
# pair rather than the toggle. Each mode therefore runs at its own documented
# defaults, plus greedy as a floor case for both.
#
#   thinking on : temperature 1.0, top_p 0.95, top_k 20
#   thinking off: temperature 0.7, top_p 0.80, top_k 20, presence_penalty 1.5
set -euo pipefail

SERVING="${SERVING:-baseline-32k-q8}"   # must match what is actually served
RESULTS="${RESULTS:-results/tier1.jsonl}"
N="${N:-3}"
TASKS="${TASKS:-tasks}"

run() { # label thinking flags...
  local label="$1" thinking="$2"; shift 2
  echo "### ${SERVING}/${label} (thinking=${thinking}) ###"
  go run ./cmd/eval -tasks "$TASKS" -n "$N" \
    -config "${SERVING}/${label}" -thinking "$thinking" \
    -results "$RESULTS" "$@" || true   # a failed task must not stop the matrix
}

run "think-on-card"   on  -temperature 1.0 -top-p 0.95 -top-k 20
run "think-off-card"  off -temperature 0.7 -top-p 0.80 -top-k 20 -presence-penalty 1.5
run "think-on-greedy"  on  -temperature 0
run "think-off-greedy" off -temperature 0

echo "### matrix complete ###"
