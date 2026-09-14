#!/usr/bin/env bash
# Install the node's three LaunchDaemons: the link address, the GPU cap as root at boot, and
# the server as the serving user with restart on failure.
#
# The plists beside this file are templates. The checkout path, the machine file, the serving
# user, that user's home and the serving PATH are substituted here rather than committed,
# because each is a fact about one machine and a committed one is wrong on the next.
#
#   sudo LOCALCODE_NODE=1 SERVE_USER=<user> ./scripts/node/install.sh
#   sudo MACHINE=config/machine.json ./scripts/node/install.sh --link-only
#
# --link-only installs the link daemon and nothing else, which is what the laptop end of the
# cable needs: it loses its address on a replug too, and `networksetup` cannot manage that
# interface either. It is not a node's configuration, so it does not ask to be told it is one.
set -euo pipefail
cd "$(dirname "$0")/../.."

# Homebrew is not on the PATH sudo builds, and bootstrap's `brew shellenv` reached only its
# own subprocess — so a session that has just bootstrapped finds no llama-server here.
# Read from the prefix rather than from the caller's profile, which sudo does not source.
[ -x /opt/homebrew/bin/brew ] && eval "$(/opt/homebrew/bin/brew shellenv)"

LINK_ONLY=0
case "${1:-}" in
  --link-only) LINK_ONLY=1 ;;
  "") ;;
  *) echo "install: unknown argument: $1 (only --link-only)" >&2; exit 2 ;;
esac

# The full install takes over the machine: the GPU cap leaves the system side a node's
# reserve rather than a desk's, and the server starts at boot and is restarted when it fails.
# Neither belongs on a machine anyone works at, and neither is undone by a reboot. An address
# on an interface ruins nothing, so --link-only does not ask.
[ "$LINK_ONLY" = 1 ] || [ "${LOCALCODE_NODE:-}" = "1" ] || {
  echo "install: this caps the GPU and serves the model at every boot, which is a dedicated node's" >&2
  echo "         configuration and would take over a machine somebody works at. Set LOCALCODE_NODE=1" >&2
  echo "         on the node to confirm that is what this machine is, or pass --link-only." >&2
  exit 2; }
[ "$(id -u)" = "0" ] || { echo "install: writing /Library/LaunchDaemons needs root; run under sudo" >&2; exit 2; }

SERVE_USER=""
SERVE_HOME=""
SERVE_PATH=""
if [ "$LINK_ONLY" = 0 ]; then
  # SUDO_USER is who ran sudo, which on the node is the serving user. Named rather than
  # assumed when it is not: the daemon reads the weights out of this user's cache.
  SERVE_USER="${SERVE_USER:-${SUDO_USER:-}}"
  [ -n "$SERVE_USER" ] || { echo "install: name the serving user in SERVE_USER" >&2; exit 2; }
  # cut rather than awk: a home with a space in it is read whole and then refused below,
  # where awk would have silently substituted its first word into the plist.
  SERVE_HOME=$(dscl . -read "/Users/$SERVE_USER" NFSHomeDirectory 2>/dev/null | cut -d' ' -f2-)
  [ -n "$SERVE_HOME" ] || { echo "install: no such user: $SERVE_USER" >&2; exit 2; }
  # launchd's own PATH carries no Homebrew, and llama-server is Homebrew's.
  SERVE_PATH="${SERVE_PATH:-/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin}"
fi

# The link is the machine file's, not this script's: scripts/node/link.sh reads it at every
# reconcile, from the file the daemon's own environment names — which is how one daemon holds
# both ends of the cable. These two are read here to say what was installed, and to fail now
# rather than in a log nobody is watching when the file names no link.
MACHINE="${MACHINE:-config/machine-m5max-36gb.json}"
machine_field() { # key
  MACHINE_FILE="$MACHINE" KEY="$1" python3 -c '
import json, os, sys
path, key = os.environ["MACHINE_FILE"], os.environ["KEY"]
try:
    print(json.load(open(path))[key])
except (OSError, ValueError, KeyError) as e:
    sys.exit(f"{path}: no {key} to read ({e})")'
}
LINK_IF=$(machine_field link_interface)
LINK_ADDR=$(machine_field link_address)

# Each of these is substituted into a sed replacement and then into XML, and both take some
# of these characters as syntax. Refusing is the fix: an escaper here would be a second
# parser for two languages, tested by nothing, standing between a boot and a served model.
for field in "checkout path=$PWD" "machine file=$MACHINE" "serving user=$SERVE_USER" \
             "home=$SERVE_HOME" "PATH=$SERVE_PATH"; do
  [ -n "${field#*=}" ] || continue   # unset in --link-only, and substituted into nothing
  case "${field#*=}" in
    *'|'*|*'&'*|*'<'*|*'>'*|*'"'*|*[[:space:]]*)
      echo "install: the ${field%%=*} holds a character sed or XML reads as syntax: ${field#*=}" >&2
      exit 2 ;;
  esac
done

# The daemons write here as two different users, so the directory is made once and owned by
# the one that is not root.
mkdir -p /Library/Logs/localcode
[ "$LINK_ONLY" = 1 ] || chown "$SERVE_USER" /Library/Logs/localcode

# The link first: an address the laptop can reach is what makes a failure of either of the
# others visible from anywhere but the node's own console, which it does not have.
LABELS="com.localcode.link com.localcode.gpucap com.localcode.serve"
[ "$LINK_ONLY" = 0 ] || LABELS="com.localcode.link"

for label in $LABELS; do
  src="scripts/node/$label.plist"
  dst="/Library/LaunchDaemons/$label.plist"
  tmp=$(mktemp /tmp/localcode-plist.XXXXXX)
  sed -e "s|__CHECKOUT__|$PWD|g" -e "s|__MACHINE__|$MACHINE|g" -e "s|__USER__|$SERVE_USER|g" \
      -e "s|__HOME__|$SERVE_HOME|g" -e "s|__PATH__|$SERVE_PATH|g" "$src" > "$tmp"
  # Rendered and compared before anything is written: rewriting an identical plist and
  # bootstrapping it again would restart the server on every run of this script, which is a
  # loaded model dropped for nothing.
  if [ -f "$dst" ] && cmp -s "$tmp" "$dst"; then
    rm -f "$tmp"
    if launchctl print "system/$label" >/dev/null 2>&1; then
      echo "install: $label unchanged and loaded" >&2
    else
      launchctl bootstrap system "$dst"
      echo "install: $label unchanged, loaded it" >&2
    fi
    continue
  fi
  mv "$tmp" "$dst"
  chown root:wheel "$dst"
  chmod 644 "$dst"
  # Bootstrapping over a loaded service fails, and a reinstall is the usual case.
  launchctl bootout "system/$label" >/dev/null 2>&1 || true
  launchctl bootstrap system "$dst"
  echo "install: installed $dst" >&2
done

echo "install: the link is $LINK_IF at $LINK_ADDR, from $MACHINE; it reconciles every 30 s" >&2
if [ "$LINK_ONLY" = 0 ]; then
  echo "install: the cap applies at boot; check /Library/Logs/localcode/gpucap.log now" >&2
  echo "install: the server logs to /Library/Logs/localcode/serve.log" >&2
fi
