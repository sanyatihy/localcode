#!/usr/bin/env bash
# Install Splash on the node from the vendor's tap, and print the version that is there.
# Splash ships as a signed Homebrew formula rather than as a Python package, so there is no
# venv here and nothing for runtimes/mlx/setup.sh's shape to mirror beyond the pins: what
# pins this runtime is the version this prints, and a row that does not carry it is not a
# reading of anything.
#
# Every step checks before it acts, as scripts/node/bootstrap.sh does, so a second run
# changes nothing and says so. An installed Splash is never upgraded here: a version that
# moves between runs is a confound, and moving it is a decision with its own rows.
set -euo pipefail

BREW="${BREW:-/opt/homebrew/bin/brew}"
# The tap is the vendor's own; the formula's keg is plain `splash`, which is what a list
# query matches once it is installed.
FORMULA="${FORMULA:-incoai/tap/splash}"

[ -x "$BREW" ] || {
  echo "splash setup: no Homebrew at $BREW — run scripts/node/bootstrap.sh first" >&2
  exit 2; }
# A non-login shell over SSH has no brew prefix on PATH, and `splash` below is resolved
# through it.
eval "$("$BREW" shellenv)"

if "$BREW" list --formula splash >/dev/null 2>&1; then
  echo "splash setup: already done — brew $FORMULA" >&2
else
  "$BREW" install "$FORMULA"
  echo "splash setup: this run did — brew install $FORMULA" >&2
fi

command -v splash >/dev/null 2>&1 || {
  echo "splash setup: the formula installed and no splash is on PATH under $("$BREW" --prefix)" >&2
  exit 1; }
echo "splash ready:"
echo "  $(splash --version) at $(command -v splash)"
