#!/usr/bin/env bash
# Hash fixed prompts at temperature zero across the admissible batch sizes, and refuse to
# go further if the batch size moves the answer.
#
# A batch size changes how a prefill is split, and floating-point reductions are not
# order-independent. If the hash moves with the batch size then a faster cell is not a
# faster way to run this model, it is a different model — and every number this repo
# recorded at 512 would describe neither. So this gates the depth sweep instead of
# accompanying it: it costs one load and two prompts a cell, against six minutes an ingest.
#
# The first cell is hashed twice. "The hashes match across cells" presupposes the hash is
# reproducible at one cell, and an instrument that cannot repeat itself cannot report a
# divergence. The repeat runs on the second load, not the second request, so it also covers
# what a reload changes.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=scripts/lib.sh
. scripts/lib.sh

BASE="${BASE:-config/tuned.env}"
CELLS="${CELLS:?CELLS is the space-separated list of admissible --ubatch-size values}"
RESULTS="${RESULTS:-results/batchfidelity.jsonl}"
[ -f "$BASE" ] || { echo "no such config: $BASE" >&2; exit 2; }
profile=$(basename "$BASE" .env)
trap stop_server EXIT
mkdir -p "$(dirname "$RESULTS")"

# The control is the first cell run a second time, so the walk is ordered: control, then
# every cell including the first.
# shellcheck disable=SC2086  # a space-separated list of batch sizes, and it must expand
set -- $CELLS
control="$1"
echo "=== fidelity across $profile cells: $* (control: $control run twice) ===" >&2

hash_cell() { # ubatch label -> prints the hash
  local ubatch="$1" label="$2"
  local cfg="/tmp/batch-$profile-ubatch-$ubatch.env"
  batch_config "$BASE" "$ubatch" "$cfg"
  stop_server
  nohup ./scripts/serve.sh "$cfg" > "/tmp/fidelity-$label.log" 2>&1 &
  wait_healthy 600 || { echo "  LOAD FAILED — see /tmp/fidelity-$label.log" >&2; return 2; }
  # -fidelity on its own: no fixture is scored here, the probes are the whole run. The
  # config label carries the batch size, so a row says which cell produced the hash.
  "$(built eval)" -fidelity -fidelity-probes prefill \
    -config "$label" -session "$profile-fidelity" -results "$RESULTS" >"/tmp/fidelity-$label.out" 2>&1 || {
      echo "  PROBE FAILED — see /tmp/fidelity-$label.out" >&2; return 2; }
  awk '/^fidelity /{print $3}' "/tmp/fidelity-$label.out"
}

expected=$(hash_cell "$control" "$profile-ubatch-$control-control")
echo "  control $control: $expected" >&2

status=0
for ubatch in "$@"; do
  got=$(hash_cell "$ubatch" "$profile-ubatch-$ubatch")
  if [ "$got" = "$expected" ]; then
    echo "  ubatch $ubatch: $got  MATCH" >&2
  else
    # Recorded, then stopped: the divergence is the result. Carrying on would score cells
    # whose numbers describe different answers, which is not a comparison.
    echo "  ubatch $ubatch: $got  DIVERGED from control $expected" >&2
    status=1
    break
  fi
done
[ "$status" = 0 ] && echo "=== every admissible cell hashes alike: the depth sweep may run ===" >&2
[ "$status" = 0 ] || echo "=== the batch size changes the answer: the depth sweep must not run ===" >&2
exit "$status"
