#!/usr/bin/env bash
# Write harness/claude-code/claude-code.env into the project-scoped settings file the
# editor extension reads, in the checkout given (default: the current one).
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
OUT="$ROOT/.claude/settings.local.json"
[ -f "$ENVFILE" ] || { echo "no env file at $ENVFILE" >&2; exit 2; }

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

{ echo '{'; echo '  "env": {'; cat "$OUT.body"; echo '  }'; echo '}'; } > "$OUT"
rm -f "$OUT.body"
echo "wrote $OUT" >&2
