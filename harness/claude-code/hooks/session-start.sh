#!/usr/bin/env bash
# SessionStart hook. Stdout is added to the session's context; stderr is not.
set -euo pipefail

# State goes under LOCALCODE_HANDOFF_DIR when it is set, and under the checkout otherwise.
# The override is what lets a session run in somebody else's repository: a handoff written
# to their root is untracked noise in a tree nobody asked us to touch.
ROOT="${LOCALCODE_HANDOFF_DIR:-${CLAUDE_PROJECT_DIR:-$PWD}}"
HANDOFF="$ROOT/HANDOFF.md"
# What a session inherits and what it will write are two files whenever a supervisor is
# carrying one session's handoff to the next. One file cannot be both: the session reads it,
# and SessionEnd then finds it already written and leaves the previous session's words in
# place. With no supervisor they are the same file, which is 0016's case unchanged.
INHERIT="${LOCALCODE_INHERIT:-$HANDOFF}"

if [ -s "$INHERIT" ]; then
  echo "The previous session in this chain left this behind:"
  echo
  cat "$INHERIT"
else
  cat <<EOF
Nothing was handed to you: the handoff is absent or empty, so you are the first session on
this box. Create it at exactly this path, in this shape:

    $HANDOFF

    # Handoff
    **Box:** the task box being worked, copied from the feature doc
    **Files:** each path that matters, and why it does
    **Tried:** what was done, and what came of it
    **Next:** the one thing to do next
EOF
fi

cat <<EOF

Keep that file current as you work, at that path and nowhere else — writing a HANDOFF.md
into the repository you are visiting leaves a file its owner did not ask for. It is the
whole of what the next session gets, since this one will not be summarised for it. Keep it
under 40 lines: every line is read again at every session start, and a session that starts
by reading a diary is the cost this replaces.
EOF
