#!/usr/bin/env bash
# The serving daemon's program (0061). com.localcode.serve is gated on the switch file, and
# launchd holds this job alive while that file is there.
#
# It stands between the daemon and scripts/serve.sh because KeepAlive implies a speculative
# launch at load: launchd runs the job once whatever the switch says, and a server started by
# that launch would be a node nobody asked to serve. Exiting 0 with the switch absent makes
# the outcome the switch's rather than launchd's.
set -euo pipefail
cd "$(dirname "$0")/../.."

# shellcheck source=scripts/lib.sh
. scripts/lib.sh

CONFIG="${1:-config/node.env}"
# What this node serves that its committed config does not: KEY="value" lines, applied over
# the config. The repository holds defaults; a served context raised on one machine is that
# machine's state and not a commit, and it survives a reboot without the checkout moving.
OVERRIDES="${SERVE_OVERRIDES:-$HOME/.config/localcode/serve.env}"
# The merged file the server is started on, so what is being served is one file a reader
# can open and the banner scripts/serve.sh prints names it.
SERVING="${SERVING:-$HOME/.local/state/localcode/serving.env}"

if [ ! -e "$SERVE_SWITCH" ]; then
  echo "serve: no $SERVE_SWITCH, so this node is stopped on purpose; create it to serve" >&2
  exit 0
fi
if [ -s "$OVERRIDES" ]; then
  mkdir -p "$(dirname "$SERVING")"
  { echo "# $CONFIG with $OVERRIDES applied by scripts/node/serve.sh; edit those, not this."
    cat "$CONFIG"; echo; echo "# --- $OVERRIDES"; cat "$OVERRIDES"; } > "$SERVING"
  echo "serve: $CONFIG with $OVERRIDES applied, as $SERVING" >&2
  exec ./scripts/serve.sh "$SERVING"
fi
exec ./scripts/serve.sh "$CONFIG"
