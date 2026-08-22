#!/usr/bin/env bash
# Score cold ingest at depth for each admissible prefill batch, at the profile's own context.
#
# Depth is the whole question. A batch size that wins on a 512-token prompt and loses at
# 32,000 has lost the case this project actually has: the editor turn that motivated the
# sweep was 36,309 tokens of prefill against 82 generated. scripts/ladder.sh already fills a
# context genuinely and reports what that cost, so a cell here is one of its rungs held at
# one batch size, and `fill_seconds` is the column that has to move.
#
# Every fill is cold, because ladder.sh restarts the server for each. A second request at
# the same depth is served from the prompt cache and would measure the cache.
#
# One pass per invocation, on purpose. Run it again for a second pass: cells interleave
# with the machine's own drift that way, where four repeats of one cell back to back would
# take the drift for a result.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=scripts/lib.sh
. scripts/lib.sh

BASE="${BASE:-config/tuned.env}"
CELLS="${CELLS:?CELLS is the space-separated list of admissible --ubatch-size values}"
OUT="${OUT:-results/batchdepth.jsonl}"
CONDITION="${CONDITION:-unattended}"
[ -f "$BASE" ] || { echo "no such config: $BASE" >&2; exit 2; }

# The context and KV type are the profile's own: this sweep moves the batch and nothing
# else, so the rung is whatever that profile serves.
CTX=$(grep '^CTX_SIZE=' "$BASE" | cut -d'"' -f2)
KV=$(grep '^CACHE_TYPE_K=' "$BASE" | cut -d'"' -f2)
profile=$(basename "$BASE" .env)
echo "=== depth sweep from $BASE: ctx=$CTX kv=$KV, batches $CELLS ===" >&2

for ubatch in $CELLS; do
  cfg="/tmp/batch-$profile-ubatch-$ubatch.env"
  batch_config "$BASE" "$ubatch" "$cfg"
  echo "=== $profile --ubatch-size $ubatch at $CTX ===" >&2
  BASE="$cfg" CELLS="$CTX:$KV" OUT="$OUT" CONDITION="$CONDITION" ./scripts/ladder.sh
done
echo "=== depth sweep complete: rows in $OUT ===" >&2
