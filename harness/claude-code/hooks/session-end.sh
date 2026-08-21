#!/usr/bin/env bash
# SessionEnd hook. Writes a handoff when the session wrote none, from the transcript
# alone — no model is called.
set -euo pipefail

payload=$(cat)
# State goes under LOCALCODE_HANDOFF_DIR when it is set, and under the checkout otherwise.
# The override is what lets a session run in somebody else's repository: a handoff written
# to their root is untracked noise in a tree nobody asked us to touch.
ROOT="${LOCALCODE_HANDOFF_DIR:-${CLAUDE_PROJECT_DIR:-$PWD}}"
mkdir -p "$ROOT" 2>/dev/null || true
HANDOFF="$ROOT/HANDOFF.md"

if [ -s "$HANDOFF" ]; then
  exit 0
fi

python3 - "$payload" "$ROOT" <<'PY'
import json, os, sys

payload, root = json.loads(sys.argv[1]), sys.argv[2]

WROTE = ("Edit", "Write", "MultiEdit", "NotebookEdit")
FILES, CALLS, SAID, WIDTH = 8, 6, 400, 100


def named(args):
    """The one argument worth showing for a call, cut to one short line."""
    for key in ("file_path", "path", "notebook_path", "command", "pattern", "url"):
        if args.get(key):
            value = str(args[key]).strip().splitlines()[0]
            if key.endswith("path"):
                value = short(value)
            return value[:WIDTH] + "…" if len(value) > WIDTH else value
    return ""


def short(path):
    """Relative to the checkout, which is where the next session stands."""
    if not os.path.isabs(path):
        return path
    rel = os.path.relpath(path, root)
    return path if rel.startswith("..") else rel


edited, read, calls, said = [], [], [], ""
try:
    lines = open(payload.get("transcript_path", "")).readlines()
except OSError:
    lines = []

for line in lines:
    try:
        row = json.loads(line)
    except ValueError:
        continue
    if row.get("type") != "assistant":
        continue
    for block in (row.get("message") or {}).get("content") or []:
        if block.get("type") == "text" and block.get("text", "").strip():
            said = block["text"].strip()
        elif block.get("type") == "tool_use":
            name, args = block.get("name", "?"), block.get("input") or {}
            calls.append("%s %s" % (name, named(args)))
            into = edited if name in WROTE else read
            if args.get("file_path"):
                into.append(short(args["file_path"]))

# Most recent first, each path once.
uniq = lambda paths: list(dict.fromkeys(reversed(paths)))[:FILES]
edited, read = uniq(edited), [p for p in uniq(read) if p not in set(uniq(edited))]

out = ["# Handoff", ""]
out += ["Extracted from the transcript when session `%s` ended (%s): it wrote no handoff,"
        % (payload.get("session_id", "?"), payload.get("reason", "?")),
        "so this records what the session did and not what it meant to do.", ""]
out += ["**Box:** not recorded — take the topmost unticked box in the feature doc.", ""]
out += ["**Files:** " + (", ".join("`%s` (edited)" % p for p in edited)
                         + (", " if edited and read else "")
                         + ", ".join("`%s`" % p for p in read) or "none touched"), ""]
out += ["**Tried:**" + ("" if calls else " nothing — no tool was called")]
out += ["- `%s`" % c for c in calls[-CALLS:]]
said = "\n".join(said.splitlines()[:5])[:SAID]
out += ["", "**Next:** " + (said or "not recorded — the session said nothing.")]

with open(os.path.join(root, "HANDOFF.md"), "w") as f:
    f.write("\n".join(out) + "\n")
PY
