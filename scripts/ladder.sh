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
set -euo pipefail

CONDITION="${CONDITION:-unlabelled}"
OUT="${OUT:-results/ceiling.jsonl}"
FILL_FRACTION="${FILL_FRACTION:-0.90}"   # leave headroom for the reply
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
  : > "$samples"
  ( while :; do ./scripts/memprobe.sh; sleep 2; done ) >> "$samples" 2>/dev/null &
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
}))
print("  -> $outcome  peak_rss=%.2f GB  wired_peak=%.2f GB  headroom_min=%.2f GB (%d samples)"
      % (max(l["llama_rss_gb"], f["llama_rss_gb"]), wired_peak, wired_headroom_min,
         len(series) - 2), file=sys.stderr)
PY
done
echo "=== ladder complete ===" >&2
