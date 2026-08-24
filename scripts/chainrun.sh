#!/usr/bin/env bash
# Drive one instruction to completion on one serving config, and score it by the fixture's
# own tests rather than by what the sessions said about themselves.
#
# The unit of comparison is a whole chain, because that is what the trade is between: a
# smaller served context is a faster decode and a lower ceiling, so it buys tokens per
# second and pays in sessions. Neither side of that is visible in a single session, and
# tokens per second is not the measure at all — a chain that is faster per token and slower
# to the answer has lost.
#
# What it holds fixed is everything but the config: the same fixture, materialised from
# scripts/chainfixture.py so the two repositories are identical by construction rather than
# by a copy somebody made; the same instruction; the same tool set, call budget and session
# bound. What varies is the file named in CONFIG.
#
# The server is restarted for each run and the fixture is fresh, because a chain served by
# a server holding the last chain's prefixes is not measuring a cold start, and a fixture
# with the last run's fixes in it is not measuring the work.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=scripts/lib.sh
. scripts/lib.sh

CONFIG="${CONFIG:?CONFIG names the serving config this chain runs on}"
LABEL="${LABEL:?LABEL names this run in the results file}"
OUT="${OUT:-results/chain.jsonl}"
WORK="${WORK:-/private/tmp/localcode-chainrun}"
SESSIONS="${SESSIONS:-20}"
CALLS="${CALLS:-200}"
SESSION_TIMEOUT="${SESSION_TIMEOUT:-45m}"
# Unset serves the ceiling the window implies, which is what a config actually runs at. Set,
# it holds two configs to one depth — decode falls with depth whatever is drafting, so a
# comparison across two contexts is measuring both unless one of them is pinned.
CEILING="${CEILING:-}"
PORT="${PORT:-8081}"
ENDPOINT="http://127.0.0.1:$PORT"
LOAD_TIMEOUT="${LOAD_TIMEOUT:-900}"

# The bounds are deliberately far above what either side needs. A chain stopped by its
# session bound or its call budget would be reporting that bound, and the question here is
# what the context costs — so the only thing allowed to end a chain is the work being done.
[ -f "$CONFIG" ] || { echo "no such config: $CONFIG" >&2; exit 2; }
mkdir -p "$(dirname "$OUT")"

# The instruction, identical on both sides and never rewritten between them. It names the
# count, because a session that cannot tell how much is left cannot write a handoff worth
# inheriting.
GOAL="Twenty tests in fixme_test.go fail, each caused by a bug in its own file. Fix the source so all twenty pass. Never edit fixme_test.go."

metrics() { # prints "prompt cached predicted", or three zeroes for a server without --metrics
  curl -s -m 10 "$ENDPOINT/metrics" 2>/dev/null | python3 -c "
import sys
want = {'llamacpp:prompt_tokens_total': 0, 'llamacpp:prompt_tokens_cached_total': 0,
        'llamacpp:tokens_predicted_total': 0}
for line in sys.stdin:
    if line.startswith('#'):
        continue
    parts = line.split()
    if len(parts) == 2 and parts[0] in want:
        want[parts[0]] = int(float(parts[1]))
print(want['llamacpp:prompt_tokens_total'], want['llamacpp:prompt_tokens_cached_total'],
      want['llamacpp:tokens_predicted_total'])" 2>/dev/null || echo "0 0 0"
}

# The launcher is built from this checkout rather than taken off PATH: an installed one is
# stamped with whichever checkout installed it, and this run has to be the code under test.
BIN="$WORK/localcode"
root="$PWD"   # this checkout, read for its harness config and hooks from inside the fixture
mkdir -p "$WORK"
go build -o "$BIN" ./cmd/localcode

rm -rf "${WORK:?}/$LABEL"
python3 scripts/chainfixture.py "$WORK/$LABEL" >&2
repo=$(cd "$WORK/$LABEL" && pwd)   # the path localcode will key its state by
total=$(grep -c '^func Test' "$repo/fixme_test.go")

# Sessions are keyed by the repository's path, so a fresh fixture directory is a fresh
# chain — but the state outlives the directory, and a stale one would hand this run the
# last one's handoff. The name is derived the way cmd/localcode derives it rather than
# matched with a glob, so this deletes that chain and never a neighbour's.
state="$HOME/.local/state/localcode/repos/$(python3 -c "
import hashlib, os, sys
p = sys.argv[1]
print(os.path.basename(p) + '-' + hashlib.sha256(p.encode()).hexdigest()[:8])" "$repo")"
rm -rf "$state"

echo "=== chain $LABEL on $CONFIG: $total bugs, up to $SESSIONS sessions ===" >&2
stop_server
nohup ./scripts/serve.sh "$CONFIG" > "/tmp/chainrun-$LABEL-serve.log" 2>&1 &
trap 'stop_server' EXIT
wait_healthy "$LOAD_TIMEOUT" || { echo "the server did not load — see /tmp/chainrun-$LABEL-serve.log" >&2; exit 2; }
served=$(served_ctx)
echo "    serving $served on $ENDPOINT" >&2

read -r p0 c0 g0 <<<"$(metrics)"
started=$SECONDS
set +e
ceiling_arg=()
[ -n "$CEILING" ] && ceiling_arg=(-ceiling "$CEILING")
( cd "$repo" && "$BIN" -checkout "$root" -endpoint "$ENDPOINT" -no-serve \
    "${ceiling_arg[@]}" \
    -calls "$CALLS" -sessions "$SESSIONS" -session-timeout "$SESSION_TIMEOUT" "$GOAL" )
chain_exit=$?
set -e
seconds=$((SECONDS - started))
read -r p1 c1 g1 <<<"$(metrics)"

# The score, and the only thing that decides whether the instruction was finished. A chain
# reports its own completion from a handoff, and a handoff is what a session said.
passed=$( (cd "$repo" && go test -v ./... 2>&1 || true) | grep -cE '^--- PASS' || true)

# One chain, because the state was wiped before this run started.
sessions_file=$(find "$state/chains" -name sessions.jsonl 2>/dev/null | head -1)
spent=0
[ -n "$sessions_file" ] && spent=$(grep -c . "$sessions_file")

LABEL="$LABEL" CONFIG="$CONFIG" GOAL="$GOAL" SESSIONS_FILE="${sessions_file:-}" python3 - <<PY >> "$OUT"
import json, os
row = {"record": "chain", "label": os.environ["LABEL"], "config": os.environ["CONFIG"],
       "served_n_ctx": json.loads('''$served'''), "goal": os.environ["GOAL"],
       "bugs": $total, "tests_passing": $passed, "finished": $passed == $total,
       "chain_exit": $chain_exit, "sessions": $spent, "wall_seconds": $seconds,
       "session_bound": $SESSIONS, "call_budget": $CALLS,
       "ceiling_pct": ${CEILING:-None},
       "prompt_tokens_ingested": $p1 - $p0, "prompt_tokens_cached": $c1 - $c0,
       "tokens_generated": $g1 - $g0}
print(json.dumps(row))
path = os.environ["SESSIONS_FILE"]
if path:
    for line in open(path):
        if line.strip():
            s = json.loads(line)
            s["record"] = "session"
            s["label"] = os.environ["LABEL"]
            print(json.dumps(s))
PY

echo "  -> $passed/$total passing after $spent sessions and ${seconds}s; ingested $((p1 - p0)), generated $((g1 - g0))" >&2
