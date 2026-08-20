#!/usr/bin/env bash
# Write harness/claude-code/claude-code.env and harness/claude-code/hooks.json into the
# project-scoped settings file the editor extension reads, in the checkout given
# (default: the current one).
#
# Two forms of one configuration exist because two clients read different things: a shell
# sources the env file, and the extension does not — it spawns its own process, which
# picks up `env` from .claude/settings.local.json in the directory it was opened on.
# Generating rather than copying is what stops the two drifting.
#
# The file is deliberately local and untracked: committed, it would route every session
# anybody opens on this repository at a 27B model on loopback, which is a decision for
# whoever opens it.
set -euo pipefail

ROOT="${1:-$PWD}"
ENVFILE="$ROOT/harness/claude-code/claude-code.env"
HOOKSFILE="$ROOT/harness/claude-code/hooks.json"
OUT="$ROOT/.claude/settings.local.json"
[ -f "$ENVFILE" ] || { echo "no env file at $ENVFILE" >&2; exit 2; }
[ -f "$HOOKSFILE" ] || { echo "no hooks file at $HOOKSFILE" >&2; exit 2; }

mkdir -p "$ROOT/.claude"
awk '
  /^[[:space:]]*(#|$)/ { next }
  {
    eq = index($0, "=")
    if (eq == 0) { print "cannot read line: " $0 > "/dev/stderr"; exit 2 }
    key = substr($0, 1, eq - 1)
    val = substr($0, eq + 1)
    gsub(/^["\x27]|["\x27]$/, "", val)
    printf "%s%s\"%s\": \"%s\"", (n++ ? ",\n" : ""), "    ", key, val
  }
  END { print "" }
' "$ENVFILE" > "$OUT.body"

{ echo '{'; echo '  "env": {'; cat "$OUT.body"; echo '  }'; echo '}'; } > "$OUT.env"
rm -f "$OUT.body"

# This script owns `env` and `hooks` and merges rather than replaces: Claude Code writes
# granted permissions into the same file.
python3 - "$OUT.env" "$HOOKSFILE" "$OUT" <<'PY'
import json, os, sys

envfile, hooksfile, outfile = sys.argv[1:4]
kept = {}
if os.path.exists(outfile):
    with open(outfile) as f:
        kept = json.load(f)
with open(envfile) as f:
    env = json.load(f)
with open(hooksfile) as f:
    hooks = json.load(f)
with open(outfile, "w") as f:
    json.dump({**kept, **env, **hooks}, f, indent=2)
    f.write("\n")
PY
rm -f "$OUT.env"
echo "wrote $OUT" >&2
