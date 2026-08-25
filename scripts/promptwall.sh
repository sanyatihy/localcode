#!/usr/bin/env bash
# Find where Claude Code refuses a prompt against the context it was declared.
#
# The harness counts a conversation through /v1/messages/count_tokens and refuses before
# sending, so the wall is the harness's arithmetic and not the server's — but the count is
# the served tokeniser's, which is why this needs the config under test actually running.
# What it reports is the largest padding that was sent and the smallest that was not, in
# tokens that server counted.
#
# A refusal costs a second and an accepted probe costs a full cold ingest, so this descends
# from an estimate in STEP-sized steps and stops at the first prompt that goes through:
# one expensive probe per run, and the bracket is STEP wide.
#
#   ./scripts/promptwall.sh                       # against the running server, declaring what it serves
#   DECLARED=45056 ./scripts/promptwall.sh        # declaring something else
#   OUTPUT=8192 STEP=128 ./scripts/promptwall.sh  # a different reservation, a tighter bracket
set -euo pipefail

ENDPOINT="${ENDPOINT:-http://127.0.0.1:8081}"
OUTPUT="${OUTPUT:-4096}"
STEP="${STEP:-250}"
TRIES="${TRIES:-40}"
RESULTS="${RESULTS:-results/promptwall.jsonl}"
TOOLS="${TOOLS:-Bash,Edit,Read,Write}"

command -v claude >/dev/null 2>&1 || { echo "promptwall: claude is not on PATH" >&2; exit 1; }
served=$(curl -s -m 5 "$ENDPOINT/props" \
  | python3 -c "import sys,json;print(json.load(sys.stdin)['default_generation_settings']['n_ctx'])") \
  || { echo "promptwall: no server at $ENDPOINT" >&2; exit 1; }
DECLARED="${DECLARED:-$served}"

work=$(mktemp -d); trap 'rm -rf "$work"' EXIT
mkdir -p "$work/cwd" "$(dirname "$RESULTS")"

# Padding of an exact token count, by detokenising a prefix and correcting against a
# re-tokenisation: a character count is not a token count, and the whole measurement is in
# tokens.
pad() {
  N="$1" ENDPOINT="$ENDPOINT" python3 -c '
import json, os, urllib.request
E, n = os.environ["ENDPOINT"], int(os.environ["N"])
def post(p, b):
    r = urllib.request.Request(E + p, data=json.dumps(b).encode(),
                               headers={"Content-Type": "application/json"})
    return json.load(urllib.request.urlopen(r, timeout=120))
words = "alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima mike".split()
toks = post("/tokenize", {"content": " ".join(words[i % len(words)] for i in range(n * 2 + 64))})["tokens"]
k, s = n, ""
for _ in range(12):
    s = post("/detokenize", {"tokens": toks[:k]})["content"]
    got = len(post("/tokenize", {"content": s})["tokens"])
    if got == n:
        break
    k = max(1, min(k + n - got, len(toks)))
print(s, end="")'
}

ingested() { curl -s -m 5 "$ENDPOINT/metrics" | awk '/^llamacpp:prompt_tokens_total/{print $2}'; }

# One probe. Prints the row and returns 0 when the harness sent the prompt.
#
# The environment is built rather than inherited: a session launched from inside Claude Code
# exports CLAUDE_CODE_* variables that change the tool set, and the tool set is half the
# preamble this measures the wall against.
probe() { # tokens
  local n="$1" before after code out start end
  pad "$n" > "$work/prompt.txt"
  printf '\n\nReply with exactly: ok\n' >> "$work/prompt.txt"
  rm -rf "$work/cfg"; mkdir -p "$work/cfg"
  before=$(ingested); start=$SECONDS
  out=$(cd "$work/cwd" && env -i PATH="$PATH" HOME="$HOME" TERM=dumb \
    ANTHROPIC_BASE_URL="$ENDPOINT" ANTHROPIC_AUTH_TOKEN=local \
    ANTHROPIC_MODEL="$model" ANTHROPIC_DEFAULT_HAIKU_MODEL="$model" \
    ANTHROPIC_DEFAULT_SONNET_MODEL="$model" ANTHROPIC_DEFAULT_OPUS_MODEL="$model" \
    CLAUDE_CODE_DISABLE_TERMINAL_TITLE=1 CLAUDE_CODE_DISABLE_CRON=1 \
    CLAUDE_CODE_DISABLE_EXPLORE_PLAN_AGENTS=1 CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1 \
    CLAUDE_CODE_MAX_CONTEXT_TOKENS="$DECLARED" CLAUDE_CODE_MAX_OUTPUT_TOKENS="$OUTPUT" \
    CLAUDE_CONFIG_DIR="$work/cfg" \
    claude --tools "$TOOLS" --allowedTools "$TOOLS" -p "$(cat "$work/prompt.txt")" \
      </dev/null 2>&1) && code=0 || code=$?
  end=$SECONDS; after=$(ingested)
  W="$DECLARED" O="$OUTPUT" N="$n" C="$code" B="$before" A="$after" K="${preamble:-0}" \
    S=$((end - start)) SERVED="$served" T="$TOOLS" OUT="$out" python3 -c '
import json, os
o = os.environ
row = {"record": "probe", "served_n_ctx": int(o["SERVED"]),
       "declared_context": int(o["W"]), "declared_output": int(o["O"]),
       "tools": o["T"], "padding_tokens": int(o["N"]),
       "sent": o["C"] == "0", "exit": int(o["C"]),
       "counted_tokens": int(o["N"]) + int(o["K"]),
       "processed_tokens": int(float(o["A"]) - float(o["B"])), "seconds": int(o["S"]),
       "said": o["OUT"].strip().splitlines()[-1][:120] if o["OUT"].strip() else ""}
print(json.dumps(row))' | tee -a "$RESULTS"
  return "$code"
}

model=$(curl -s -m 5 "$ENDPOINT/props" \
  | python3 -c "import sys,json,os;print(os.path.basename(json.load(sys.stdin).get('model_path','local')))")

echo "promptwall: $ENDPOINT serves $served, declaring $DECLARED with $OUTPUT reserved" >&2

# The preamble first: what the harness spends before the padding, which is what turns a
# padding size into the count the wall is actually against.
probe 1 || { echo "promptwall: even an empty prompt is refused at $DECLARED" >&2; exit 1; }

# What that probe cost is the preamble: it ran cold, so the tokens the server processed are
# the tokens the harness counted, less the one of padding.
preamble=$(tail -n 1 "$RESULTS" \
  | python3 -c "import json,sys;r=json.load(sys.stdin);print(r['processed_tokens'] - 1)")

# Above every wall measured and not far above: the largest prompt the harness sent was 0.79
# of the declared context less its reservation, so 0.85 of that starts on the refusing side
# and every step down costs a second until the first one goes through.
reservation=$(( OUTPUT > 4096 ? OUTPUT : 4096 ))
start=$(( (DECLARED - reservation) * 85 / 100 - preamble ))
for i in $(seq 0 "$TRIES"); do
  n=$(( start - i * STEP ))
  [ "$n" -lt 1 ] && break
  if probe "$n"; then
    echo "promptwall: sent at $n, refused at $(( n + STEP )) — the wall is between them" >&2
    exit 0
  fi
done
echo "promptwall: no prompt was sent in $TRIES steps below $start" >&2
exit 1
