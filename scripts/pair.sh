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

# Scorer flags a runtime forces on the pair. EVAL_ARGS goes to both sides, because a setting
# one side cannot take has to leave the other too or the pair measures the setting: Splash
# refuses presence_penalty, so its pairs send 0 to both. CANDIDATE_EVAL_ARGS is for a toggle
# the candidate spells differently, such as thinking off as `-reasoning-effort none`.
EVAL_ARGS="${EVAL_ARGS:-}"
CANDIDATE_EVAL_ARGS="${CANDIDATE_EVAL_ARGS:-}"

# Readiness is judged by the pid this script started, not by lib.sh's pgrep for
# llama-server. MTPLX's command line is just its interpreter and mlx_lm's is just python,
# so the pattern finds nothing: past the health grace every such load was being called dead
# while its process was alive and still loading, which made the 600 s timeout unreachable.
# screen.sh overrides this for the same reason. Unconditional, because the override is also
# stricter for llama.cpp — a stray server somebody else left running satisfies the pattern
# and says nothing about the one this run launched.
server_pid=""
server_alive() { [ -n "$server_pid" ] && kill -0 "$server_pid" 2>/dev/null; }

# wait_gone ends the server this run started and waits for the memory back. lib.sh's
# stop_server does this for llama-server by pattern and so returns immediately for a runtime
# it cannot see — which let the next side start loading while the previous one still held
# twenty gigabytes, and a pair whose two sides overlap in memory measures neither. The
# bound and the SIGKILL after it are stop_server's, for the same reason: an 18 GB process
# does not exit on a fixed sleep.
wait_gone() { # pid
  [ -n "${1:-}" ] || return 0
  kill "$1" 2>/dev/null || true
  local deadline=$((SECONDS + 90))
  while kill -0 "$1" 2>/dev/null; do
    [ $SECONDS -ge $deadline ] && { kill -9 "$1" 2>/dev/null || true; sleep 3; break; }
    sleep 1
  done
  sleep 2
}

# The side with the mechanism off runs first, so a candidate never benefits from a
# machine the baseline warmed and the caches it left behind.
for side in "$BASELINE" "$CANDIDATE"; do
  label=$(basename "$side" .env)
  serve="./scripts/serve.sh"; port="$PORT"
  extra="$EVAL_ARGS"
  if [ "$side" = "$CANDIDATE" ]; then
    serve="$CANDIDATE_SERVE"; port="$CANDIDATE_PORT"; extra="$EVAL_ARGS $CANDIDATE_EVAL_ARGS"
  fi
  ENDPOINT="http://127.0.0.1:$port"
  echo "=== $label ===" >&2
  stop_server
  # shellcheck disable=SC2086
  nohup $serve "$side" > "/tmp/pair-$label.log" 2>&1 &
  server_pid=$!
  if ! wait_healthy 600; then
    echo "  LOAD FAILED — see /tmp/pair-$label.log" >&2
    wait_gone "$server_pid"
    exit 2
  fi
  if [ -n "$TASK" ]; then set -- -task "$TASK"; else set -- -tasks "$TASKS"; fi
  # shellcheck disable=SC2086 # $extra is a flag list, split on purpose
  "$(built eval)" "$@" -n "$REPEATS" -config "$label" -session "$SESSION" -endpoint "$ENDPOINT" \
    -stream -fidelity -results "$RESULTS" -thinking "$THINKING" -sampling-profile "$SAMPLING" \
    $extra || true
  # By pid and then by pattern: the next side must not start while this one still holds
  # memory, and stop_server alone cannot wait for a process it cannot see.
  wait_gone "$server_pid"
  server_pid=""
  stop_server
done

"$(built report)" -results "$RESULTS" -baseline "$(basename "$BASELINE" .env)"
