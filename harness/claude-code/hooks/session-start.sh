#!/usr/bin/env bash
# SessionStart hook. Stdout is added to the session's context; stderr is not.
set -euo pipefail

ROOT="${CLAUDE_PROJECT_DIR:-$PWD}"
HANDOFF="$ROOT/HANDOFF.md"

if [ -s "$HANDOFF" ]; then
  echo "The previous session in this checkout left this in HANDOFF.md:"
  echo
  cat "$HANDOFF"
else
  cat <<'EOF'
Nothing was handed to you: HANDOFF.md is absent or empty, so you are the first session on
this box. Create it at the root of the checkout in this shape:

    # Handoff
    **Box:** the task box being worked, copied from the feature doc
    **Files:** each path that matters, and why it does
    **Tried:** what was done, and what came of it
    **Next:** the one thing to do next
EOF
fi

cat <<'EOF'

Keep HANDOFF.md current as you work. It is untracked scratch and it is the whole of what
the next session gets — this one will not be summarised for it. Keep it under 40 lines:
every line is read again at every session start, and a session that starts by reading a
diary is the cost this replaces.
EOF
