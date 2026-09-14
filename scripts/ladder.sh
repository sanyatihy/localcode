#!/usr/bin/env bash
# Drive each ladder rung to a genuinely full context and record what it cost.
#
# Filling the context is the whole point. Allocation at load time understates the peak, and
# a config that loads and then dies at 30k is exactly the failure this measures.
#
# The rungs are derived, never written down. scripts/rungs.sh computes them from what this
# machine actually has and reports which of memory or time binds; a hand-written rung is a
# fact about one laptop, which docs/VISION.md names as the first defect to fix rather than a
# value to update. Each cell is generated from BASE with only the context and KV type moved,
# so a rung cannot drift from the config the project serves.
#
# CONDITION labels the machine state, because the same config has two answers: with a desktop
# in use (attended) and with the machine to itself (unattended).
#
# Wired memory is sampled *during* the fill. Metal's buffers peak while the context fills, and
# a probe taken after curl returns can miss it — which is how a cell that exhausted the GPU
# wired limit was recorded as `ok` with nothing in the row to contradict the desktop freezing.
#
# `outcome` says whether the model finished; `desktop_verdict` says whether the machine stayed
# usable while it did. Collapsing them is the mistake this ladder already made once.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=scripts/lib.sh
. scripts/lib.sh

CONDITION="${CONDITION:-unlabelled}"
OUT="${OUT:-results/ceiling.jsonl}"
BASE="${BASE:-config/tuned.env}"
FILL_FRACTION="${FILL_FRACTION:-0.90}"   # leave headroom for the reply

# Which cells to walk, as ctx:kv pairs. Empty derives one per rung at the base config's KV
# type, which is the ladder's default question; naming them asks about a band or a KV type
# instead. A re-walk usually asks about one band — 64k alone is 13 minutes.
CELLS="${CELLS-}"
if [ -z "$CELLS" ]; then
  kv=$(grep '^CACHE_TYPE_K=' "$BASE" | cut -d'"' -f2)
  CELLS=$(./scripts/rungs.sh | sed "s/$/:$kv/" | tr '\n' ' ')
fi

# The desktop rule, written before the runs so it is a rule and not a preference. Units are
# fractions of one core, from consecutive WindowServer CPU-time readings.
#
# Basis: attended with no model loaded measures 0.17-0.47 cores here, the low end a near-static
# screen and the high end an editor actively rendering. Saturation sits well above that, stall
# well below. Both are failures — a compositor pinned at a core cannot keep up, and one doing
# nothing is not drawing.
#
# Known limit: passive CPU cannot tell "nothing to draw" from "stuck and not drawing", so the
# verdict means something only when somebody is driving the machine. Unattended cells report
# `not_applicable` rather than a pass they did not earn.
DESK_SATURATED="${DESK_SATURATED:-0.90}"
DESK_STALLED="${DESK_STALLED:-0.02}"
DESK_SUSTAIN_SECONDS="${DESK_SUSTAIN_SECONDS:-30}"
mkdir -p "$(dirname "$OUT")"

# Record what else is resident before any cell runs. A condition label is a claim about
# the machine; this is the evidence for it. It also captures the measuring apparatus — an
# editor driving the ladder sits inside the footprint it is measuring, so a "hard ceiling"
# taken with an IDE open is conservative by however much that IDE holds.
#
# Values reach python through the environment rather than string interpolation: process
# names contain characters that would otherwise terminate the quoting and corrupt the row.
APPARATUS=$(ps -Ao rss,comm | awk '$1 > 102400 && $2 !~ /llama-server/ && $2 != "COMM" {
    n=split($2,p,"/"); printf "%s%.2fGB %s", (c++?"; ":""), $1/1048576, p[n]}')
# Two numbers, because one of them lies. The summed RSS is kept for continuity with rows
# already recorded, but it counts every shared page once per resident process and so
# overstates the footprint — by about 1.5x on a state measured both ways. Anonymous memory
# is what actually competes with the model for the 32 GB, and is the one to read.
APPARATUS_TOTAL=$(ps -Ao rss,comm | awk '$2 !~ /llama-server/ {s+=$1} END {printf "%.2f", s/1048576}')
APPARATUS_ANON=$(apparatus_anonymous_gb)
# The compositor's rate before any model is loaded. Recorded per run so a verdict carries
# the basis it was judged against, rather than inheriting a number measured once by hand
# and quoted thereafter — the machine's baseline is not a constant.
DESK_BASELINE=$(desk_baseline)

export APPARATUS APPARATUS_TOTAL APPARATUS_ANON CONDITION DESK_BASELINE
# shellcheck disable=SC2086  # a space-separated list of ctx:kv pairs, and it must expand
set -- $CELLS
echo "walking $# cell(s) from $BASE: $*" >&2
echo "apparatus before any cell: ${APPARATUS_ANON} GB anonymous (summed RSS says ${APPARATUS_TOTAL}, which overcounts shared pages)" >&2
if [ "$DESK_BASELINE" = "not_applicable" ]; then
  echo "desktop baseline before any cell: not_applicable — no compositor on this machine" >&2
else
  echo "desktop baseline before any cell: ${DESK_BASELINE} cores" >&2
fi
python3 -c '
import json, os
print(json.dumps({"condition": os.environ["CONDITION"], "record": "apparatus",
                  "anonymous_gb": float(os.environ["APPARATUS_ANON"]),
                  "resident_gb": float(os.environ["APPARATUS_TOTAL"]),
                  "desktop_baseline_cores": (None if os.environ["DESK_BASELINE"] == "not_applicable"
                                             else float(os.environ["DESK_BASELINE"])),
                  "processes": os.environ["APPARATUS"]}))' >> "$OUT"

# shellcheck disable=SC2086
for cell in $CELLS; do
  ctx="${cell%%:*}"; kv="${cell##*:}"
  name="ladder-${ctx}-${kv}"
  cfg="/tmp/$name.env"
  cell_config "$BASE" "$ctx" "$kv" "$cfg"
  # The prefill batch is the base's, since cell_config moves only the context and the KV
  # type. It goes on the row and into the name: a base that sets it produces a different
  # measurement at the same rung, and two rows called `ladder-32768-q8_0` cannot be told
  # apart. A base that leaves it at llama.cpp's default names nothing and keeps the row
  # shape every earlier ladder wrote.
  # `|| true` on both: under `pipefail` a grep that matches nothing fails the whole
  # substitution, and a base that leaves the batch at llama.cpp's default is the normal
  # case rather than an error.
  ubatch=$(grep '^UBATCH_SIZE=' "$cfg" | cut -d'"' -f2 || true)
  batch=$(grep '^BATCH_SIZE=' "$cfg" | cut -d'"' -f2 || true)
  if [ -n "$ubatch" ]; then name="$name-ub$ubatch"; fi
  target=$(python3 -c "print(int($ctx * $FILL_FRACTION))")

  echo "=== $name  ctx=$ctx kv=$kv  filling to ~$target tokens ===" >&2
  stop_server
  before=$(memprobe_json)

  nohup ./scripts/serve.sh "$cfg" >"/tmp/ladder-$name.log" 2>&1 &
  if ! wait_healthy 300; then
    echo '  LOAD FAILED' >&2
    python3 -c "
import json,sys
print(json.dumps({'condition':'$CONDITION','cell':'$name','ctx':$ctx,'kv':'$kv',
 'outcome':'load_failed','before':json.loads('''$before''')}))" >> "$OUT"
    continue
  fi
  actual=$(served_ctx)
  slots=$(served_slots); [ "$slots" = "null" ] && slots=1
  per_slot=$(per_slot_ctx "$ctx")
  if [ "$slots" -le 1 ]; then
    if [ "$actual" != "$ctx" ]; then
      echo "  ABORT: server reports n_ctx=$actual, expected $ctx — refusing to measure the wrong config" >&2
      python3 -c "
import json
print(json.dumps({'condition':'$CONDITION','cell':'$name','ctx':$ctx,'kv':'$kv',
 'outcome':'wrong_config_served','served_n_ctx':$actual}))" >> "$OUT"
      continue
    fi
  elif [ "$per_slot" = "ambiguous" ] || [ "$per_slot" = "null" ]; then
    echo "  ABORT: server reports n_ctx=$actual over $slots slots, which is neither $ctx per slot nor $ctx in total — refusing to measure a context it cannot name" >&2
    python3 -c "
import json
print(json.dumps({'condition':'$CONDITION','cell':'$name','ctx':$ctx,'kv':'$kv','slots':$slots,
 'outcome':'wrong_config_served','served_n_ctx':'$actual'}))" >> "$OUT"
    continue
  fi
  # The fill is sized against what one slot holds, which is what the server just said rather
  # than what the config asked for. On one slot they are the same number.
  target=$(python3 -c "print(int($per_slot * $FILL_FRACTION))")
  [ "$slots" -gt 1 ] && echo "  $slots slots, filling each to ~$target tokens" >&2
  loaded=$(memprobe_json)

  # One request per slot, each whose prompt genuinely occupies that slot's context. The
  # seeds differ so no two slots send the same prefix: a server that served the second from
  # what the first ingested would report a rung nobody paid for. Slot 1 keeps seed 11, so a
  # one-slot machine sends exactly the prompt every earlier ladder sent.
  slot=1
  while [ "$slot" -le "$slots" ]; do
    python3 - "$target" "$slot" > "/tmp/ladder-fill-$slot.json" <<'PY'
import json, random, sys
n, slot = int(sys.argv[1]), int(sys.argv[2])
random.seed(10 + slot)
vocab = ["session","token","refresh","handler","request","context","buffer","index",
         "commit","parser","value","result","config","client","server","stream"]
body = " ".join(random.choice(vocab) for _ in range(n))
json.dump({"messages":[{"role":"user","content":body+"\n\nReply with the single word OK."}],
           "max_tokens":8,"temperature":0,
           "chat_template_kwargs":{"enable_thinking":False}}, open('/dev/stdout','w'))
PY
    slot=$(( slot + 1 ))
  done
  # A detached sampler for the duration of the request. Two seconds costs a vm_stat and a
  # ps; a short cell still yields a few samples and a 64k cell yields hundreds.
  samples="/tmp/ladder-wired-$name.jsonl"
  desk="/tmp/ladder-desk-$name.jsonl"
  : > "$samples"; : > "$desk"
  ( while :; do
      memprobe_json >> "$samples"
      # The compositor is macOS's, and its probe shells out to tools a headless box does not
      # have: sampling it there would kill this loop rather than record a verdict.
      if darwin; then ./scripts/deskprobe.sh >> "$desk"; fi
      sleep 2
    done ) 2>/dev/null &
  sampler=$!

  # Every slot at once, because a server with four slots holding one filled context is not
  # the state four developers put it in: the KV of all four is what has to fit.
  fill_start=$SECONDS
  pids=""
  slot=1
  while [ "$slot" -le "$slots" ]; do
    ( curl -s -m 3600 -o "/tmp/ladder-fill-resp-$slot.json" -w '%{http_code}' \
        "$ENDPOINT/v1/chat/completions" \
        -H 'Content-Type: application/json' -d @"/tmp/ladder-fill-$slot.json" \
        > "/tmp/ladder-fill-http-$slot" || echo 000 > "/tmp/ladder-fill-http-$slot" ) &
    pids="$pids $!"
    slot=$(( slot + 1 ))
  done
  for pid in $pids; do wait "$pid" || true; done
  fill_seconds=$((SECONDS - fill_start))
  http=$(cat "/tmp/ladder-fill-http-1")
  kill "$sampler" 2>/dev/null || true
  wait "$sampler" 2>/dev/null || true
  filled=$(memprobe_json)

  # A rung passes only when every slot filled: one failure among four is the whole cell's
  # answer, not three quarters of one.
  outcome=ok
  slot=1
  while [ "$slot" -le "$slots" ]; do
    if grep -q '"error"' "/tmp/ladder-fill-resp-$slot.json" 2>/dev/null; then outcome=rejected; fi
    if [ "$(cat "/tmp/ladder-fill-http-$slot")" = "000" ]; then outcome=request_failed; fi
    slot=$(( slot + 1 ))
  done
  pgrep -f llama-server >/dev/null || outcome=died

  python3 - <<PY >> "$OUT"
import json, sys
b=json.loads('''$before'''); l=json.loads('''$loaded'''); f=json.loads('''$filled''')

# The end-point probes are part of the series, not separate from it: if the sampler was
# starved, or the fill was shorter than one interval, they are all there is.
series=[l, f]
try:
    series += [json.loads(x) for x in open("$samples") if x.strip()]
except (OSError, ValueError):
    pass
# Both are Metal's and both are None off macOS, so the series is filtered rather than
# reduced over: a cell that reports no wired figure has no wired peak, which is not a peak
# of zero.
peaks=[p["wired_gb"] for p in series if p["wired_gb"] is not None]
wired_peak=max(peaks) if peaks else None
room=[p["wired_headroom_gb"] for p in series if p["wired_headroom_gb"] is not None]
wired_headroom_min=min(room) if room else None
gb=lambda v: "unknown" if v is None else "%.2f GB" % v

# WindowServer's progress while the model was under load. Rates are derived from the
# series rather than in the probe, so the sampling interval stays visible in the raw data.
# The rule itself lives in scripts/deskverdict.py: a screen judges a cell the same way.
sys.path.insert(0, "scripts")
import deskverdict
desk=deskverdict.load("$desk")
desktop=deskverdict.verdict(desk, "$CONDITION", $DESK_SATURATED, $DESK_STALLED,
                            $DESK_SUSTAIN_SECONDS)

# Prompt throughput is the discriminator. Under memory pressure a process's resident size is
# clamped by what physically fits rather than by what the config wants, so it stops
# distinguishing configs exactly when the answer matters; time to ingest a full context does
# not. One figure per slot: what a slot was charged is what a developer on it waited for.
rates=[]
for i in range(1, $slots + 1):
    try:
        rates.append(json.load(open("/tmp/ladder-fill-resp-%d.json" % i))
                     .get("timings", {}).get("prompt_per_second", 0))
    except (OSError, ValueError):
        rates.append(0)

print(json.dumps({
  "condition": "$CONDITION", "cell": "$name", "ctx": $ctx, "kv": "$kv",
  "slots": $slots, "per_slot_ctx": $per_slot,
  "ubatch": ${ubatch:-None}, "batch": ${batch:-None},
  "fill_target_tokens": $target, "outcome": "$outcome", "http": "$http",
  "fill_seconds": $fill_seconds, "prompt_per_second": rates[0],
  **({"prompt_per_second_slots": rates} if len(rates) > 1 else {}),
  "before": b, "loaded": l, "filled": f,
  "swap_delta_load_mb": round(l["swap_used_mb"]-b["swap_used_mb"], 1),
  "swap_delta_fill_mb": round(f["swap_used_mb"]-l["swap_used_mb"], 1),
  "swap_delta_total_mb": round(f["swap_used_mb"]-b["swap_used_mb"], 1),
  "peak_rss_gb": max(l["llama_rss_gb"], f["llama_rss_gb"]),
  "free_at_peak_gb": min(l["free_gb"], f["free_gb"]),
  "wired_peak_gb": (round(wired_peak, 3) if wired_peak is not None else None),
  "wired_headroom_min_gb": (round(wired_headroom_min, 3) if wired_headroom_min is not None else None),
  "wired_limit_gb": f["wired_limit_gb"], "wired_limit_source": f["wired_limit_source"],
  "wired_samples": len(series) - 2,
  **desktop,
}))
print("  -> $outcome  desktop=%s  peak_rss=%.2f GB  wired_peak=%s  headroom_min=%s"
      % (desktop["desktop_verdict"], max(l["llama_rss_gb"], f["llama_rss_gb"]),
         gb(wired_peak), gb(wired_headroom_min)),
      file=sys.stderr)
PY
done
echo "=== ladder complete ===" >&2
