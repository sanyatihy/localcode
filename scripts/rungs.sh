#!/usr/bin/env bash
# Print the context rungs this machine should be laddered over, one per line.
#
# Every rung in the first ladder was a fact about a 32 GB machine, written by hand. A
# 128 GB machine moves all of them at once, and a hand-edited list is a list that gets
# forgotten — so the rungs are computed from what the machine actually has.
#
# The arithmetic is deliberately crude, because its job is to bracket the interesting
# range rather than predict it. 0003 measured the KV cost that matters here and it came
# out four times lower than the config-derived figure, so a precise model would be a
# false one; what is wanted is a top rung high enough that the ladder can fail.
set -euo pipefail

MODEL_PATH="${MODEL_PATH:-}"                  # GGUF to size the weights from
KB_PER_TOKEN="${KB_PER_TOKEN:-35}"            # measured 31-37 on Qwen3.8-27B q8_0 KV
RESERVE_GB="${RESERVE_GB:-8}"                 # OS, editor, browser — never the model's
MAX_CTX="${MAX_CTX:-262144}"                  # the model's own limit; no point above it
MAX_INGEST_MIN="${MAX_INGEST_MIN:-20}"        # a rung nobody will wait for is not a measurement

# TOTAL_GB overrides detection so a machine can be planned before it is owned — the
# 128 GB upgrade changes which constraint binds, and that is worth knowing in advance.
total_gb="${TOTAL_GB:-$(( $(sysctl -n hw.memsize) / 1073741824 ))}"

# Weights dominate the floor. Measure them from the file when we can, rather than
# assuming: a different quant or model changes this more than anything else here.
if [ -n "$MODEL_PATH" ] && [ -f "$MODEL_PATH" ]; then
  # -L follows the symlink. The HuggingFace cache stores snapshots as links into
  # blobs/, and BSD stat reports the link's own size without it — which reads as a
  # 0 GB model and silently inflates the headroom estimate.
  weights_gb=$(( $(stat -L -f%z "$MODEL_PATH") / 1073741824 ))
else
  weights_gb="${WEIGHTS_GB:-16}"
fi

headroom_gb=$(( total_gb - RESERVE_GB - weights_gb ))
if [ "$headroom_gb" -lt 1 ]; then
  echo "rungs: no headroom — ${total_gb}GB total, ${weights_gb}GB weights, ${RESERVE_GB}GB reserved" >&2
  exit 2
fi

# What memory allows.
max_fit=$(( headroom_gb * 1048576 / KB_PER_TOKEN ))
[ "$max_fit" -gt "$MAX_CTX" ] && max_fit="$MAX_CTX"

# What time allows, which on this hardware is the smaller number by a wide margin.
# 0003 measured the prompt rate decaying with depth — 109 tok/s at 8k to 75 at 64k —
# so cost grows faster than context. Extrapolated linearly from those points and
# floored, because a rung that takes an hour to ingest cannot be laddered over even
# when it fits comfortably in memory.
max_time=$(python3 -c "
budget = $MAX_INGEST_MIN * 60
ctx, last = 8192, 8192
while ctx <= $MAX_CTX:
    rate = max(20.0, 109.0 - 0.000593 * (ctx - 8192))
    if ctx / rate > budget: break
    last = ctx; ctx *= 2
print(last)")

max_ctx_eff=$max_fit
[ "$max_time" -lt "$max_ctx_eff" ] && max_ctx_eff="$max_time"

binding=memory
[ "$max_time" -lt "$max_fit" ] && binding="ingest time"
echo "rungs: ${total_gb}GB total, ${weights_gb}GB weights, ${RESERVE_GB}GB reserved" >&2
echo "       memory allows ~${max_fit} tokens; ${MAX_INGEST_MIN} min of ingest allows ~${max_time}" >&2
echo "       -> ${binding} binds, laddering to ${max_ctx_eff}" >&2

# Powers of two from 8k up to the first rung at or past the estimate. Going one past is
# the point: a ladder whose every rung holds has not measured a ceiling.
ctx=8192
while [ "$ctx" -le "$MAX_CTX" ]; do
  echo "$ctx"
  [ "$ctx" -ge "$max_ctx_eff" ] && break
  ctx=$(( ctx * 2 ))
done
