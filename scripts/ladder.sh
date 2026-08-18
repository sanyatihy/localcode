#!/usr/bin/env bash
# Drive each ladder config to a genuinely full context and record what it cost.
#
# Filling the context is the whole point. Allocation at load time understates the peak,
# and a config that loads and then dies at 30k is exactly the failure this measures. So
# every cell sends a prompt sized to the context it claims to serve.
#
# CONDITION labels the machine state, because the same config has two answers: with a
# desktop in use (attended) and with the machine to itself (unattended). Recording it is
# what keeps the two from being averaged together later.
#
# Wired memory is sampled *during* the fill, not around it. Metal's buffers peak while the
# context fills, and a probe taken after curl returns can miss it — which is how a cell that
# exhausted the GPU wired limit was recorded as `ok` with nothing in the row to contradict
# the desktop freezing while it ran.
#
# The cell's outcome and the desktop's verdict are separate columns on purpose. `outcome`
# says whether the model finished; `desktop_verdict` says whether the machine stayed usable
# while it did. Collapsing them is the mistake this ladder already made once.
set -euo pipefail

CONDITION="${CONDITION:-unlabelled}"
OUT="${OUT:-results/ceiling.jsonl}"
FILL_FRACTION="${FILL_FRACTION:-0.90}"   # leave headroom for the reply

# The desktop rule, written before the runs so it is a rule and not a preference. Units are
# fractions of one core, derived from consecutive WindowServer CPU-time readings.
#
# Basis: attended, an editor rendering and no model loaded, measures 0.28-0.47 cores on
# this machine over repeated samples.
# Saturation is set well above that; stall well below. Both are failures — a compositor
# pinned at a core cannot keep up, and one doing nothing is not drawing.
#
# Known limit, and the reason 0014 still wants a scripted UI interaction: passive CPU cannot
# tell "nothing to draw" from "stuck and not drawing". So the verdict is only meaningful
# when someone is driving the machine, and unattended cells report `not_applicable` rather
# than a pass they did not earn. Direction is unverified until a known-bad cell is walked;
# the full series is recorded so the threshold can be reset from evidence.
DESK_SATURATED="${DESK_SATURATED:-0.90}"
DESK_STALLED="${DESK_STALLED:-0.02}"
DESK_SUSTAIN_SECONDS="${DESK_SUSTAIN_SECONDS:-30}"
mkdir -p "$(dirname "$OUT")"

# An 18 GB process does not exit on a fixed sleep. Poll until it is genuinely gone,
# or the next server fails to bind and the health check passes against the old one —
# which silently measures the previous config under the next config's name.
stop_server() {
  pkill -f llama-server 2>/dev/null || true
  local deadline=$((SECONDS + 90))
  while pgrep -f llama-server >/dev/null; do
    [ $SECONDS -ge $deadline ] && { pkill -9 -f llama-server 2>/dev/null || true; sleep 3; break; }
    sleep 1
  done
  sleep 2
}

# The guard that would have caught the above: ask the server what it is serving and
# refuse to measure if it disagrees with the config we launched.
served_ctx() {
  curl -s -m 5 http://127.0.0.1:8080/props 2>/dev/null \
    | python3 -c "import sys,json;print(json.load(sys.stdin)['default_generation_settings']['n_ctx'])" 2>/dev/null || echo 0
}

wait_healthy() { # seconds
  local deadline=$((SECONDS + $1))
  while [ $SECONDS -lt $deadline ]; do
    curl -s -m 2 http://127.0.0.1:8080/health 2>/dev/null | grep -q '"ok"' && return 0
    pgrep -f llama-server >/dev/null || return 1
    sleep 3
  done
  return 1
}

# Record what else is resident before any cell runs. A condition label is a claim about
# the machine; this is the evidence for it. It also captures the measuring apparatus — an
# editor driving the ladder sits inside the footprint it is measuring, so a "hard ceiling"
# taken with an IDE open is conservative by however much that IDE holds.
#
# Values reach python through the environment rather than string interpolation: process
# names contain characters that would otherwise terminate the quoting and corrupt the row.
APPARATUS=$(ps -Ao rss,comm | awk '$1 > 102400 && $2 !~ /llama-server/ && $2 != "COMM" {
    n=split($2,p,"/"); printf "%s%.2fGB %s", (c++?"; ":""), $1/1048576, p[n]}')
APPARATUS_TOTAL=$(ps -Ao rss,comm | awk '$2 !~ /llama-server/ {s+=$1} END {printf "%.2f", s/1048576}')
export APPARATUS APPARATUS_TOTAL CONDITION
echo "apparatus resident before any cell: ${APPARATUS_TOTAL} GB" >&2
python3 -c '
import json, os
print(json.dumps({"condition": os.environ["CONDITION"], "record": "apparatus",
                  "resident_gb": float(os.environ["APPARATUS_TOTAL"]),
                  "processes": os.environ["APPARATUS"]}))' >> "$OUT"

for cfg in config/ladder-*.env; do
  name=$(basename "$cfg" .env)
  ctx=$(grep '^CTX_SIZE=' "$cfg" | cut -d'"' -f2)
  kv=$(grep '^CACHE_TYPE_K=' "$cfg" | cut -d'"' -f2)
  target=$(python3 -c "print(int($ctx * $FILL_FRACTION))")

  echo "=== $name  ctx=$ctx kv=$kv  filling to ~$target tokens ===" >&2
  stop_server
  before=$(./scripts/memprobe.sh)

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
  if [ "$actual" != "$ctx" ]; then
    echo "  ABORT: server reports n_ctx=$actual, expected $ctx — refusing to measure the wrong config" >&2
    python3 -c "
import json
print(json.dumps({'condition':'$CONDITION','cell':'$name','ctx':$ctx,'kv':'$kv',
 'outcome':'wrong_config_served','served_n_ctx':$actual}))" >> "$OUT"
    continue
  fi
  loaded=$(./scripts/memprobe.sh)

  # One request whose prompt genuinely occupies the context.
  python3 - "$target" > /tmp/ladder-fill.json <<'PY'
import json, random, sys
random.seed(11)
vocab = ["session","token","refresh","handler","request","context","buffer","index",
         "commit","parser","value","result","config","client","server","stream"]
n = int(sys.argv[1])
body = " ".join(random.choice(vocab) for _ in range(n))
json.dump({"messages":[{"role":"user","content":body+"\n\nReply with the single word OK."}],
           "max_tokens":8,"temperature":0,
           "chat_template_kwargs":{"enable_thinking":False}}, open('/dev/stdout','w'))
PY
  # A detached sampler for the duration of the request. Two seconds costs a vm_stat and a
  # ps; a short cell still yields a few samples and a 64k cell yields hundreds.
  samples="/tmp/ladder-wired-$name.jsonl"
  desk="/tmp/ladder-desk-$name.jsonl"
  : > "$samples"; : > "$desk"
  ( while :; do
      ./scripts/memprobe.sh >> "$samples"
      ./scripts/deskprobe.sh >> "$desk"
      sleep 2
    done ) 2>/dev/null &
  sampler=$!

  fill_start=$SECONDS
  http=$(curl -s -m 3600 -o /tmp/ladder-fill-resp.json -w '%{http_code}' \
        http://127.0.0.1:8080/v1/chat/completions \
        -H 'Content-Type: application/json' -d @/tmp/ladder-fill.json || echo 000)
  fill_seconds=$((SECONDS - fill_start))
  kill "$sampler" 2>/dev/null || true
  wait "$sampler" 2>/dev/null || true
  filled=$(./scripts/memprobe.sh)

  # Prompt throughput is the discriminator. Under memory pressure `ps rss` is clamped by
  # what physically fits rather than by what the config wants, so it stops distinguishing
  # configs exactly when the answer matters. Time to ingest a full context does not.
  prompt_rate=$(python3 -c "
import json
try:
    d=json.load(open('/tmp/ladder-fill-resp.json'))
    print(d.get('timings',{}).get('prompt_per_second',0))
except Exception:
    print(0)")

  outcome=ok
  grep -q '"error"' /tmp/ladder-fill-resp.json 2>/dev/null && outcome=rejected
  [ "$http" = "000" ] && outcome=request_failed
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
wired_peak=max(p["wired_gb"] for p in series)
wired_headroom_min=min(p["wired_headroom_gb"] for p in series)

# WindowServer's progress while the model was under load. Rates are derived here rather
# than in the probe so the sampling interval stays visible in the raw series.
desk=[]
try:
    desk=sorted((json.loads(x) for x in open("$desk") if x.strip()), key=lambda r: r["t"])
except (OSError, ValueError):
    pass

def cores(a, b):
    span=b["t"]-a["t"]
    return None if span <= 0 else (b["windowserver_cpu_seconds"]-a["windowserver_cpu_seconds"])/span

steps=[c for c in (cores(a, b) for a, b in zip(desk, desk[1:])) if c is not None]

# Every window of at least the sustain length. A single spike is the compositor doing its
# job; a spike that does not end is the compositor losing.
sustained=[]
for i in range(len(desk)):
    k=i+1
    while k < len(desk) and desk[k]["t"] - desk[i]["t"] < $DESK_SUSTAIN_SECONDS:
        k += 1
    if k < len(desk):
        c=cores(desk[i], desk[k])
        if c is not None:
            sustained.append(c)

condition="$CONDITION"
if not condition.startswith("attended"):
    verdict="not_applicable"
elif not sustained:
    verdict="insufficient_samples"
elif max(sustained) >= $DESK_SATURATED:
    verdict="fail_saturated"
elif min(sustained) <= $DESK_STALLED:
    verdict="fail_stalled"
else:
    verdict="pass"

print(json.dumps({
  "condition": "$CONDITION", "cell": "$name", "ctx": $ctx, "kv": "$kv",
  "fill_target_tokens": $target, "outcome": "$outcome", "http": "$http",
  "fill_seconds": $fill_seconds, "prompt_per_second": $prompt_rate,
  "before": b, "loaded": l, "filled": f,
  "swap_delta_load_mb": round(l["swap_used_mb"]-b["swap_used_mb"], 1),
  "swap_delta_fill_mb": round(f["swap_used_mb"]-l["swap_used_mb"], 1),
  "swap_delta_total_mb": round(f["swap_used_mb"]-b["swap_used_mb"], 1),
  "peak_rss_gb": max(l["llama_rss_gb"], f["llama_rss_gb"]),
  "free_at_peak_gb": min(l["free_gb"], f["free_gb"]),
  "wired_peak_gb": round(wired_peak, 3),
  "wired_headroom_min_gb": round(wired_headroom_min, 3),
  "wired_limit_gb": f["wired_limit_gb"], "wired_limit_source": f["wired_limit_source"],
  "wired_samples": len(series) - 2,
  "desktop_verdict": verdict,
  "ws_cpu_peak_cores": round(max(steps), 3) if steps else None,
  "ws_cpu_sustained_max_cores": round(max(sustained), 3) if sustained else None,
  "ws_cpu_sustained_min_cores": round(min(sustained), 3) if sustained else None,
  "ws_span_seconds": round(desk[-1]["t"] - desk[0]["t"], 1) if len(desk) > 1 else 0,
  "ws_samples": len(desk),
  "desktop_thresholds": {"saturated_cores": $DESK_SATURATED, "stalled_cores": $DESK_STALLED,
                         "sustain_seconds": $DESK_SUSTAIN_SECONDS},
}))
print("  -> $outcome  desktop=%s  peak_rss=%.2f GB  wired_peak=%.2f GB  headroom_min=%.2f GB"
      % (verdict, max(l["llama_rss_gb"], f["llama_rss_gb"]), wired_peak, wired_headroom_min),
      file=sys.stderr)
PY
done
echo "=== ladder complete ===" >&2
