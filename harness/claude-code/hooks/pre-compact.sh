#!/usr/bin/env bash
# PreCompact: refuse, and record that it was asked for.
#
# Exit 2 is the only exit code that blocks compaction; every other one lets it proceed.
# Neither stdout nor stderr reaches the model here — both go to the debug log — so the
# record is a file rather than something said to the session.
#
# Both triggers are refused. A manual /compact re-ingests the conversation exactly as an
# automatic one does, so honouring it would spend the generation this exists to save; the
# trigger is recorded rather than branched on, and a summary a human wants is HANDOFF.md.
set -euo pipefail

payload=$(cat)
ROOT="${CLAUDE_PROJECT_DIR:-$PWD}"
LOG="$ROOT/results/precompact.jsonl"

# Best-effort, and deliberately not under `set -e`: the refusal is what the design rests
# on, so a record that could not be written must not turn a refusal into a compaction.
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
