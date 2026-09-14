#!/usr/bin/env bash
# Install the node's two LaunchDaemons: the GPU cap as root at boot, and the server as the
# serving user with restart on failure.
#
# The plists beside this file are templates. The checkout path, the serving user, that
# user's home and the serving PATH are substituted here rather than committed, because a
# path is a fact about one machine and a committed one is wrong on the next.
#
#   sudo LOCALCODE_NODE=1 SERVE_USER=<user> ./scripts/node/install.sh
set -euo pipefail
cd "$(dirname "$0")/../.."

# This takes over the machine: the GPU cap leaves the system side a node's reserve rather
# than a desk's, and the server starts at boot and is restarted when it fails. Neither
# belongs on a machine anyone works at, and neither is undone by a reboot.
[ "${LOCALCODE_NODE:-}" = "1" ] || {
  echo "install: this caps the GPU and serves the model at every boot, which is a dedicated node's" >&2
  echo "         configuration and would take over a machine somebody works at. Set LOCALCODE_NODE=1" >&2
  echo "         on the node to confirm that is what this machine is." >&2
  exit 2; }
[ "$(id -u)" = "0" ] || { echo "install: writing /Library/LaunchDaemons needs root; run under sudo" >&2; exit 2; }

# SUDO_USER is who ran sudo, which on the node is the serving user. Named rather than
# assumed when it is not: the daemon reads the weights out of this user's cache.
SERVE_USER="${SERVE_USER:-${SUDO_USER:-}}"
[ -n "$SERVE_USER" ] || { echo "install: name the serving user in SERVE_USER" >&2; exit 2; }
# cut rather than awk: a home with a space in it is read whole and then refused below, where
# awk would have silently substituted its first word into the plist.
SERVE_HOME=$(dscl . -read "/Users/$SERVE_USER" NFSHomeDirectory 2>/dev/null | cut -d' ' -f2-)
[ -n "$SERVE_HOME" ] || { echo "install: no such user: $SERVE_USER" >&2; exit 2; }
# launchd's own PATH carries no Homebrew, and llama-server is Homebrew's.
SERVE_PATH="${SERVE_PATH:-/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin}"

# Each of these is substituted into a sed replacement and then into XML, and both take some
# of these characters as syntax. Refusing is the fix: an escaper here would be a second
# parser for two languages, tested by nothing, standing between a boot and a served model.
for field in "checkout path=$PWD" "serving user=$SERVE_USER" "home=$SERVE_HOME" "PATH=$SERVE_PATH"; do
  case "${field#*=}" in
    *'|'*|*'&'*|*'<'*|*'>'*|*'"'*|*[[:space:]]*)
      echo "install: the ${field%%=*} holds a character sed or XML reads as syntax: ${field#*=}" >&2
      exit 2 ;;
  esac
done

# The daemons write here as two different users, so the directory is made once and owned by
# the one that is not root.
mkdir -p /Library/Logs/localcode
chown "$SERVE_USER" /Library/Logs/localcode

for label in com.localcode.gpucap com.localcode.serve; do
  src="scripts/node/$label.plist"
  dst="/Library/LaunchDaemons/$label.plist"
  sed -e "s|__CHECKOUT__|$PWD|g" -e "s|__USER__|$SERVE_USER|g" \
      -e "s|__HOME__|$SERVE_HOME|g" -e "s|__PATH__|$SERVE_PATH|g" "$src" > "$dst"
  chown root:wheel "$dst"
  chmod 644 "$dst"
  # Bootstrapping over a loaded service fails, and a reinstall is the usual case.
  launchctl bootout "system/$label" >/dev/null 2>&1 || true
  launchctl bootstrap system "$dst"
  echo "installed $dst" >&2
done

echo "install: the cap applies at boot; check /Library/Logs/localcode/gpucap.log now" >&2
echo "install: the server logs to /Library/Logs/localcode/serve.log" >&2
