#!/usr/bin/env bash
# PreCompact hook. Exit 2 blocks the compaction; nothing else does.
set -euo pipefail

payload=$(cat)
ROOT="${CLAUDE_PROJECT_DIR:-$PWD}"
LOG="$ROOT/results/precompact.jsonl"

# Best-effort: a record that fails must not cost the refusal.
mkdir -p "$ROOT/results" 2>/dev/null || true
python3 -c '
import datetime, json, sys

fired = json.loads(sys.argv[1])
fired["at"] = datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds")
with open(sys.argv[2], "a") as f:
    f.write(json.dumps(fired) + "\n")
' "$payload" "$LOG" || true

echo "compaction refused: end this session and hand off through HANDOFF.md instead" >&2
exit 2
