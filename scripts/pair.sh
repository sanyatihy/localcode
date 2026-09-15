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
cd "$(dirname "$0")/.."
# shellcheck source=scripts/lib.sh
. scripts/lib.sh

BASELINE="${BASELINE:-config/tuned.env}"
CANDIDATE="${CANDIDATE:?CANDIDATE is the serving config under test}"
SESSION="${SESSION:?SESSION labels the pair, and only rows sharing one may be divided}"
TASKS="${TASKS:-tasks}"
# TASK runs one fixture instead of a directory, which is what a depth curve wants: each
# point is its own pair, and mixing depths into one session would average the curve away.
TASK="${TASK:-}"
REPEATS="${REPEATS:-3}"
RESULTS="${RESULTS:-results/pair.jsonl}"
PORT="${PORT:-8081}"
ENDPOINT="http://127.0.0.1:$PORT"
THINKING="${THINKING:-off}"
SAMPLING="${SAMPLING:-nonthinking}"

# A candidate that is not llama.cpp is started by a script of its own and answers on a port
# of its own. `scripts/screen.sh` takes the serve command as arguments for exactly this
# reason; here the baseline is always this project's server, so only the candidate side
# needs the override. It is tracked by pid because a runtime that sets its own process
# title cannot be found by the library's pgrep.
CANDIDATE_SERVE="${CANDIDATE_SERVE:-./scripts/serve.sh}"
CANDIDATE_PORT="${CANDIDATE_PORT:-$PORT}"

# The side with the mechanism off runs first, so a candidate never benefits from a
# machine the baseline warmed and the caches it left behind.
for side in "$BASELINE" "$CANDIDATE"; do
  label=$(basename "$side" .env)
  serve="./scripts/serve.sh"; port="$PORT"
  if [ "$side" = "$CANDIDATE" ]; then serve="$CANDIDATE_SERVE"; port="$CANDIDATE_PORT"; fi
  ENDPOINT="http://127.0.0.1:$port"
  echo "=== $label ===" >&2
  stop_server
  # shellcheck disable=SC2086
  nohup $serve "$side" > "/tmp/pair-$label.log" 2>&1 &
  server_pid=$!
  if ! wait_healthy 600; then
    echo "  LOAD FAILED — see /tmp/pair-$label.log" >&2
    kill "$server_pid" 2>/dev/null || true
    exit 2
  fi
  if [ -n "$TASK" ]; then set -- -task "$TASK"; else set -- -tasks "$TASKS"; fi
  "$(built eval)" "$@" -n "$REPEATS" -config "$label" -session "$SESSION" -endpoint "$ENDPOINT" \
    -stream -fidelity -results "$RESULTS" -thinking "$THINKING" -sampling-profile "$SAMPLING" || true
  # By pid as well as by pattern: the next side must not measure this one's memory, and
  # stop_server's pgrep cannot see a runtime whose command line is just its interpreter.
  kill "$server_pid" 2>/dev/null || true
  stop_server
done

"$(built report)" -results "$RESULTS" -baseline "$(basename "$BASELINE" .env)"
