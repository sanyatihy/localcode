#!/usr/bin/env bash
# Walk --ubatch-size up from llama.cpp's default until this machine refuses, and stop there.
#
# The admissible range is found rather than written down. The compute buffer grows with the
# physical batch and the GPU's wired limit is where that stops fitting, so the top of the
# range is a reading off this laptop — the same reason scripts/rungs.sh derives its rungs
# instead of carrying a list somebody typed.
#
# Each cell is screened, not laddered. scripts/screen.sh already holds the whole rule — it
# loaded, it generated a token, it did not swap, it left the compositor drawing — and 0014
# established that a config the machine cannot carry fails at the load rather than partway
# through a fill.
#
# The smoke prompt is sized to the batch under test. A physical batch is dispatched at full
# size only by a prompt with that many tokens in it, so a fixed 512-token smoke would screen
# every cell at 512 and find nothing.
#
# --batch-size follows only where it would otherwise clamp. It is the logical ceiling and
# llama.cpp raises the physical batch no higher than it; below 2048 it is already above the
# value under test and moving it would change a second thing.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=scripts/lib.sh
. scripts/lib.sh

BASE="${BASE:-config/tuned.env}"
CONDITION="${CONDITION:-unattended}"
OUT="${OUT:-results/batch.jsonl}"
START="${START:-512}"   # llama.cpp's own --ubatch-size default
[ -f "$BASE" ] || { echo "no such config: $BASE" >&2; exit 2; }

CTX=$(grep '^CTX_SIZE=' "$BASE" | cut -d'"' -f2)
[ -n "$CTX" ] || { echo "$BASE sets no CTX_SIZE" >&2; exit 2; }
profile=$(basename "$BASE" .env)
mkdir -p "$(dirname "$OUT")"

echo "=== batch sweep from $BASE: ctx=$CTX, walking --ubatch-size up from $START ===" >&2

ubatch="$START"
while :; do
  # A batch larger than the context can never be filled, so it is not a configuration this
  # profile has: the walk has reached the context rather than the allocator, and says so.
  if [ "$ubatch" -gt "$CTX" ]; then
    echo "stopped at $ubatch: larger than this profile's context ($CTX)" >&2
    break
  fi
  label="$profile-ubatch-$ubatch"
  cfg="/tmp/batch-$label.env"
  batch_config "$BASE" "$ubatch" "$cfg"

  # The prompt is capped at 90% of the context, which leaves room for the reply. Above that
  # the batch is bounded by the context rather than by the allocator, and the cell above
  # would be measuring the wrong thing.
  smoke=$ubatch
  cap=$(( CTX * 9 / 10 ))
  [ "$smoke" -gt "$cap" ] && smoke="$cap"

  LABEL="$label" CTX="$CTX" CONDITION="$CONDITION" OUT="$OUT" SMOKE_TOKENS="$smoke" \
    ./scripts/screen.sh ./scripts/serve.sh "$cfg"

  read -r admissible reason <<<"$(python3 - "$OUT" <<'PY'
import json, sys
row = json.loads(open(sys.argv[1]).read().rstrip().rsplit("\n", 1)[-1])
print(json.dumps(row["admissible"]), row["admissible_reason"])
PY
)"
  # A refusal ends the range; a void does not. screen.sh voids a cell the machine swapped
  # under, and that cell was not measured rather than found inadmissible — so the walk still
  # stops, but it must not be read as having found the ceiling.
  if [ "$admissible" = "null" ]; then
    echo "stopped at --ubatch-size $ubatch: NOT MEASURED ($reason) — everything above it is unknown, not inadmissible" >&2
    break
  fi
  if [ "$admissible" != "true" ]; then
    echo "stopped at --ubatch-size $ubatch: refused ($reason)" >&2
    break
  fi
  ubatch=$(( ubatch * 2 ))
done
echo "=== batch sweep complete: rows in $OUT ===" >&2
