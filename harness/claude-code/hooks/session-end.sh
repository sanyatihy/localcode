#!/usr/bin/env bash
# SessionEnd: write a handoff when the session did not.
#
# The model is told to keep HANDOFF.md current, and this is the net under that instruction
# rather than the mechanism — a handoff the session wrote knows what it meant to do, and
# this one can only know what it did.
#
# No model is called: a transcript is read for what it records mechanically. Spending
# generation on a summary is the cost the whole arrangement exists to avoid, and spending
# it after the session has ended would buy even less than compaction does.
set -euo pipefail

payload=$(cat)
ROOT="${CLAUDE_PROJECT_DIR:-$PWD}"
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
    """The one argument worth showing for a call, whatever the tool calls it.

    Cut to one short line: a shell command can be a whole heredoc, and this file is read
    again at every session start.
    """
    for key in ("file_path", "path", "notebook_path", "command", "pattern", "url"):
        if args.get(key):
            value = str(args[key]).strip().splitlines()[0]
            if key.endswith("path"):
                value = short(value)
            return value[:WIDTH] + "…" if len(value) > WIDTH else value
    return ""


def short(path):
    """Paths relative to the checkout, which is the only place the next session stands."""
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

# Most recent first, and each path once: what a session touched last is what the next one
# most likely needs, and the same file read eleven times is one fact.
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
