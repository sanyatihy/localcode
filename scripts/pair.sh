#!/usr/bin/env bash
# Measure a candidate against its baseline back to back, on one machine state.
#
# Two configs cannot be loaded at once on 32 GB, so "paired" here means consecutive
# rather than simultaneous: same tasks, same sampling, same session label, one server
# stopped before the next starts. That is what makes the two sides divisible — an
# unpaired before-and-after measures host drift as well as the change, and a published
# harness that scores its own unmodified tree at 0.994 rather than 1.000 is the size of
# the noise being cancelled here.
#
# The scorer streams, because decode is the half of the clock a speculative decoder can
# move and only the client can see where prefill ended, and it hashes fixed greedy probes
# on each side: lossless is the premise of the comparison, so a mismatch takes the ratio
# away rather than appearing beside it. `scripts/screen.sh` decides whether a config may
# run at all; this decides whether it is worth running.
set -euo pipefail

BASELINE="${BASELINE:-config/tuned.env}"
CANDIDATE="${CANDIDATE:?CANDIDATE is the serving config under test}"
SESSION="${SESSION:?SESSION labels the pair, and only rows sharing one may be divided}"
TASKS="${TASKS:-tasks}"
REPEATS="${REPEATS:-3}"
RESULTS="${RESULTS:-results/pair.jsonl}"
PORT="${PORT:-8081}"
THINKING="${THINKING:-off}"
SAMPLING="${SAMPLING:-nonthinking}"

stop_server() {
  pkill -f llama-server 2>/dev/null || true
  local deadline=$((SECONDS + 90))
  while pgrep -f llama-server >/dev/null; do
    [ $SECONDS -ge $deadline ] && { pkill -9 -f llama-server 2>/dev/null || true; sleep 3; break; }
    sleep 1
  done
  sleep 2
}

wait_healthy() {
  local deadline=$((SECONDS + 600)) grace=$((SECONDS + 20))
  while [ $SECONDS -lt $deadline ]; do
    [ "$(curl -s -m 2 -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PORT/health" 2>/dev/null)" = "200" ] && return 0
    if [ $SECONDS -ge $grace ] && ! pgrep -f llama-server >/dev/null; then return 1; fi
    sleep 3
  done
  return 1
}

# The side with the mechanism off runs first, so a candidate never benefits from a
# machine the baseline warmed and the caches it left behind.
for side in "$BASELINE" "$CANDIDATE"; do
  label=$(basename "$side" .env)
  echo "=== $label ===" >&2
  stop_server
  nohup ./scripts/serve.sh "$side" > "/tmp/pair-$label.log" 2>&1 &
  if ! wait_healthy; then
    echo "  LOAD FAILED — see /tmp/pair-$label.log" >&2
    exit 2
  fi
  go run ./cmd/eval -tasks "$TASKS" -n "$REPEATS" -config "$label" -session "$SESSION" \
    -stream -fidelity -results "$RESULTS" -thinking "$THINKING" -sampling-profile "$SAMPLING" || true
done
stop_server

go run ./cmd/report -results "$RESULTS" -baseline "$(basename "$BASELINE" .env)"
