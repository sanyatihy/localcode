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

if [ ! -e "$SERVE_SWITCH" ]; then
  echo "serve: no $SERVE_SWITCH, so this node is stopped on purpose; create it to serve" >&2
  exit 0
fi
exec ./scripts/serve.sh "$CONFIG"
