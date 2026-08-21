#!/usr/bin/env bash
# Decide whether a served config is admissible on this machine, for the price of a load.
#
# 0014 established that the desktop freezes as a cell loads rather than partway through
# its fill, so a verdict costs a load plus ninety seconds of sampling instead of a full
# ingest — the difference between a minute and thirteen. That is what makes screening a
# candidate before spending a suite pass on it affordable, and 0017 needs it twice.
#
# Runtime-agnostic, because the candidates are not all llama.cpp: the serve command is
# passed as arguments and the config file it reads stays the single source of truth for
# how a measurement was produced. This script adds no serving flags.
#
# Health is not the criterion. A server can answer /health with 200 while every Metal command
# buffer fails out of memory and the first real request returns 500 — measured here on a
# draft-model config that did not fit. So the screen asks for one token, and a config that
# cannot generate is inadmissible whatever its memory numbers say.
#
# What it does not measure: a runtime that allocates its KV cache per request rather than
# reserving it at load shows its weights here and not its context. Screening the same build
# at two contexts is what separates the two — a reservation moves the peak, a per-request
# cache does not — so read a candidate's two rows together rather than either alone.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=scripts/lib.sh
. scripts/lib.sh

LABEL="${LABEL:?LABEL names this candidate in the results file}"
CTX="${CTX:?CTX is the context the config claims to serve}"
CONDITION="${CONDITION:-unlabelled}"
PORT="${PORT:-8081}"
ENDPOINT="http://127.0.0.1:$PORT"
PROC="${PROC:-llama-server}"          # pgrep pattern for sweeping a leftover server away
STOP_CMD="${STOP_CMD:-}"

# A runtime that sets its own process title cannot be found by pattern — MTPLX's command line
# is just the Python binary — so this screen tracks the server it launched by pid rather than
# by the library's pgrep. STOP_CMD is the same allowance for stopping it.
server_alive() { kill -0 "${server_pid:-0}" 2>/dev/null; }
SECONDS_TO_SAMPLE="${SECONDS_TO_SAMPLE:-90}"
# How big the request that precedes the window is. A runtime that reserves its KV at load is
# judged by the load; one that allocates per request is not judged at all until a prompt
# occupies the context, and the same screen has to cover both. Unset means one line.
SMOKE_TOKENS="${SMOKE_TOKENS:-}"
LOAD_TIMEOUT="${LOAD_TIMEOUT:-600}"
OUT="${OUT:-results/screen.jsonl}"
[ $# -ge 1 ] || { echo "usage: LABEL=... CTX=... $0 <serve command...>" >&2; exit 2; }
mkdir -p "$(dirname "$OUT")"

server_rss_gb() {
  ps -o rss= -p "$server_pid" 2>/dev/null | awk '{printf "%.3f", $1/1048576}'
}

stop_server
before=$(./scripts/memprobe.sh)
baseline=$(desk_baseline)

echo "=== screening $LABEL  ctx=$CTX  condition=$CONDITION ===" >&2
echo "    $*" >&2
echo "    desktop baseline before load: $baseline cores" >&2

# Sampling starts before the launch, not after the load: the freeze this screens for begins
# as the weights are wired, so a sampler started once the server is healthy has missed it.
samples="/tmp/screen-wired-$LABEL.jsonl"; desk="/tmp/screen-desk-$LABEL.jsonl"
: > "$samples"; : > "$desk"
( while :; do ./scripts/memprobe.sh >> "$samples"; ./scripts/deskprobe.sh >> "$desk"; sleep 2; done ) 2>/dev/null &
sampler=$!
trap 'kill "$sampler" 2>/dev/null || true' EXIT

load_start=$SECONDS
nohup "$@" > "/tmp/screen-$LABEL.log" 2>&1 &
server_pid=$!
if wait_healthy "$LOAD_TIMEOUT"; then
  outcome=ok
  load_seconds=$((SECONDS - load_start))
  loaded=$(./scripts/memprobe.sh)
  rss=$(server_rss_gb)
  served=$(served_ctx)
  if [ "$served" != "null" ] && [ "$served" != "$CTX" ]; then outcome=wrong_config_served; fi
  # One token, before the sampling window: what is being screened is a server that works,
  # and the window should cover the machine in the state a suite would find it.
  python3 -c "
import json, os, random, sys
n=int(os.environ.get('SMOKE_TOKENS') or 0)
if n:
    random.seed(11)
    vocab=['session','token','refresh','handler','request','context','buffer','index',
           'commit','parser','value','result','config','client','server','stream']
    prompt=' '.join(random.choice(vocab) for _ in range(n))+chr(10)*2+'Reply with the single word OK.'
else:
    prompt='Reply with the single word OK.'
body={'messages':[{'role':'user','content':prompt}],'max_tokens':8,'temperature':0}
if os.environ.get('SMOKE_MODEL'): body['model']=os.environ['SMOKE_MODEL']
json.dump(body, sys.stdout)" > /tmp/screen-smoke-req.json
  smoke_start=$SECONDS
  smoke_http=$(curl -s -m 600 -o /tmp/screen-smoke-$LABEL.json -w '%{http_code}' \
      "$ENDPOINT/v1/chat/completions" \
      -H 'Content-Type: application/json' -d @/tmp/screen-smoke-req.json || echo 000)
  smoke_seconds=$((SECONDS - smoke_start))
  smoke=$(python3 -c "
import json
try:
    d=json.load(open('/tmp/screen-smoke-$LABEL.json'))
    print('ok' if d.get('choices') else 'error')
except Exception:
    print('error')")
  [ "$smoke_http" = "200" ] || smoke=error
  echo "    loaded in ${load_seconds}s, smoke=$smoke ($smoke_http) after ${smoke_seconds}s at ${SMOKE_TOKENS:-0} prompt tokens, sampling ${SECONDS_TO_SAMPLE}s" >&2
  sleep "$SECONDS_TO_SAMPLE"
else
  outcome=load_failed
  load_seconds=$((SECONDS - load_start))
  loaded=$before; rss=0; served=null; smoke=not_reached; smoke_http=000; smoke_seconds=0
  echo "    LOAD FAILED after ${load_seconds}s — see /tmp/screen-$LABEL.log" >&2
fi
after=$(./scripts/memprobe.sh)
kill "$sampler" 2>/dev/null || true; wait "$sampler" 2>/dev/null || true
kill "$server_pid" 2>/dev/null || true
stop_server

SERVE_CMD="$*" python3 - <<PY >> "$OUT"
import json, os, sys
sys.path.insert(0, "scripts")
import deskverdict

before=json.loads('''$before'''); loaded=json.loads('''$loaded'''); after=json.loads('''$after''')
series=[before, loaded, after]
try:
    series += [json.loads(x) for x in open("$samples") if x.strip()]
except (OSError, ValueError):
    pass
# The load is what is being judged, so the peak is taken over the whole window including it.
wired_peak=max(p["wired_gb"] for p in series)
headroom_min=min(p["wired_headroom_gb"] for p in series)
desktop=deskverdict.verdict(deskverdict.load("$desk"), "$CONDITION")

swap_grew=round(after["swap_used_mb"]-before["swap_used_mb"], 1)
outcome="$outcome"

# The rule, fixed here rather than read off each result. Attended, the desktop decides and
# 0014's verdict is the whole criterion. Unattended there is no desktop to lose, so what
# remains is whether the machine carried it at all: a run that swapped is void per the
# vision, not slow, and a load that failed is a result rather than a mishap.
if outcome != "ok":
    admissible, why = False, outcome
elif "$smoke" != "ok":
    admissible, why = False, "cannot_generate"
elif swap_grew > 0:
    admissible, why = None, "void_swapped"
elif desktop["desktop_verdict"] in ("fail_saturated", "fail_stalled"):
    admissible, why = False, desktop["desktop_verdict"]
elif desktop["desktop_verdict"] == "insufficient_samples":
    admissible, why = None, "insufficient_samples"
elif desktop["desktop_verdict"] == "not_applicable":
    admissible, why = True, "loaded_no_swap"
else:
    admissible, why = True, desktop["desktop_verdict"]

row={"record": "screen", "condition": "$CONDITION", "label": "$LABEL", "ctx": $CTX,
     "serve": os.environ["SERVE_CMD"], "proc": "$PROC", "port": $PORT,
     "outcome": outcome, "served_n_ctx": json.loads('''$served'''), "load_seconds": $load_seconds,
     "smoke": "$smoke", "smoke_http": "$smoke_http", "smoke_seconds": $smoke_seconds,
     "smoke_prompt_tokens": ${SMOKE_TOKENS:-0},
     "screen_seconds": $SECONDS_TO_SAMPLE,
     "admissible": admissible, "admissible_reason": why,
     "before": before, "loaded": loaded, "after": after,
     "swap_delta_load_mb": round(loaded["swap_used_mb"]-before["swap_used_mb"], 1),
     "swap_delta_total_mb": swap_grew,
     "server_rss_gb": float("$rss"),
     "wired_at_load_gb": loaded["wired_gb"],
     "wired_peak_gb": round(wired_peak, 3),
     "wired_headroom_min_gb": round(headroom_min, 3),
     "wired_limit_gb": after["wired_limit_gb"], "wired_limit_source": after["wired_limit_source"],
     "wired_samples": len(series) - 3,
     "desktop_baseline_cores": $baseline,
     **desktop}
print(json.dumps(row))
print("  -> %s  admissible=%s (%s)  wired_peak=%.2f GB  headroom_min=%.2f GB  desktop=%s"
      % (outcome, admissible, why, wired_peak, headroom_min, desktop["desktop_verdict"]),
      file=sys.stderr)
PY
