#!/usr/bin/env bash
# Take a clean macOS to an installed checkout that can serve. Run on the node, as the
# serving user, in an SSH session where sudo works — the owner types the password, and
# nothing here adds a sudoers rule or a passwordless route.
#
# Every step checks before it acts, so a second run changes nothing and says so. That is the
# point of it: the node is set up by running this rather than by remembering what was typed.
# It is fetched on its own before any checkout exists, so it assumes nothing around it and
# clones the repository itself.
#
#   curl -fsSLO https://raw.githubusercontent.com/sanyatihy/localcode/main/scripts/node/bootstrap.sh
#   sudo -v && LOCALCODE_NODE=1 bash bootstrap.sh
set -euo pipefail

REPO="${REPO:-https://github.com/sanyatihy/localcode.git}"
CHECKOUT="${CHECKOUT:-$HOME/Developer/localcode}"
LOCALCODE_REF="${LOCALCODE_REF:-main}"
BREW="${BREW:-/opt/homebrew/bin/brew}"
MACHINE="${MACHINE:-config/machine-m5max-36gb.json}"

[ "${LOCALCODE_NODE:-}" = "1" ] || {
  echo "bootstrap: this installs a toolchain, a model server and this project into the home of" >&2
  echo "           the user running it, which is a dedicated node's configuration. Set" >&2
  echo "           LOCALCODE_NODE=1 on the node to confirm that is what this machine is." >&2
  exit 2; }
[ "$(id -u)" != "0" ] || {
  echo "bootstrap: run this as the serving user, not root — Homebrew refuses root, and the" >&2
  echo "           checkout and the weights cache belong to the user that serves. It calls" >&2
  echo "           sudo where a step needs it." >&2
  exit 2; }
sudo -v || { echo "bootstrap: needs a session where sudo works; run it from an interactive SSH login" >&2; exit 2; }

# What this run changed and what was already so, printed at the end: a second run that says
# it skipped everything is the evidence that it is idempotent.
DID=""
SKIPPED=""
# The revision the checkout ended this run on, which is what the installed launcher is
# compared against: a rebuild that changes nothing is still a rebuild somebody has to read.
HEAD_REV=""
did() { DID="${DID}  - ${1}
"; echo "bootstrap: $1" >&2; }
skipped() { SKIPPED="${SKIPPED}  - ${1}
"; echo "bootstrap: already done — $1" >&2; }

# ensure_line appends a line to a shell profile once. grep before append, because a profile
# that sources Homebrew four times is what running a setup twice used to produce.
ensure_line() { # file line
  local file="$1" line="$2"
  [ -f "$file" ] || : > "$file"
  # A profile whose last line has no newline would take the append glued onto it, and the
  # exact-line test below would then never match again.
  [ ! -s "$file" ] || [ -z "$(tail -c1 "$file")" ] || echo >> "$file"
  if grep -qxF "$line" "$file"; then
    skipped "$file carries: $line"
  else
    printf '%s\n' "$line" >> "$file"
    did "appended to $file: $line"
  fi
}

step_command_line_tools() {
  # unverified on the node
  if xcode-select -p >/dev/null 2>&1; then
    skipped "Command Line Tools at $(xcode-select -p)"
    return 0
  fi
  # `xcode-select --install` opens a dialog, and the node has no screen and nobody at it.
  # The marker file is what makes softwareupdate list the Command Line Tools packages at all.
  local marker=/tmp/.com.apple.dt.CommandLineTools.installondemand.in-progress label
  # The marker is what makes softwareupdate list those packages, and a marker left behind
  # makes every later softwareupdate run list them too. Removed on every way out of here,
  # including the failures below.
  trap 'sudo rm -f "$marker"' RETURN
  sudo touch "$marker"
  # stderr stays visible: softwareupdate reports a refused or unreachable catalogue there,
  # and the empty label below would otherwise be the only symptom.
  label=$(softwareupdate -l |
    sed -n 's/^ *\* *Label: *\(Command Line Tools.*\)$/\1/p' | sort -V | tail -1)
  if [ -z "$label" ]; then
    echo "bootstrap: softwareupdate lists no Command Line Tools package to install" >&2
    exit 1
  fi
  sudo softwareupdate -i "$label" --verbose
  xcode-select -p >/dev/null 2>&1 || {
    echo "bootstrap: installed $label and xcode-select still finds no developer directory" >&2
    exit 1; }
  did "installed $label"
}

step_homebrew() {
  if [ -x "$BREW" ]; then
    skipped "Homebrew at $BREW"
  else
    # NONINTERACTIVE is what keeps the official installer from asking for RETURN, which
    # nobody is here to press; it still calls sudo, which the check above cached.
    NONINTERACTIVE=1 /bin/bash -c \
      "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    [ -x "$BREW" ] || { echo "bootstrap: the Homebrew installer finished and $BREW is not there" >&2; exit 1; }
    did "installed Homebrew"
  fi
  eval "$("$BREW" shellenv)"
  ensure_line "$HOME/.zprofile" "eval \"\$($BREW shellenv)\""
}

step_packages() {
  local formula
  for formula in go python llama.cpp; do
    if "$BREW" list --formula "$formula" >/dev/null 2>&1; then
      skipped "brew $formula"
    else
      "$BREW" install "$formula"
      did "brew install $formula"
    fi
  done
  # claude-code@latest, not claude-code: the plain cask lags the releases by weeks (2.1.236
  # against 2.1.270 on 2026-09-14), and a node driven by an older Claude Code than the
  # laptop's is a comparison of two harnesses. A node still carrying the plain cask is moved.
  if "$BREW" list --cask claude-code@latest >/dev/null 2>&1; then
    skipped "brew cask claude-code@latest"
  else
    if "$BREW" list --cask claude-code >/dev/null 2>&1; then
      "$BREW" uninstall --cask claude-code
      did "brew uninstall --cask claude-code (the lagging cask)"
    fi
    "$BREW" install --cask claude-code@latest
    did "brew install --cask claude-code@latest"
  fi
}

step_architecture_check() {
  # A Homebrew llama.cpp built before the model's architecture landed serves nothing, and it
  # fails at load rather than at install — README's check, run where it cannot be forgotten.
  local lib count
  lib="$("$BREW" --prefix)/lib/libllama.dylib"
  [ -f "$lib" ] || { echo "bootstrap: $lib is not there, so llama.cpp did not install" >&2; exit 1; }
  count=$(strings "$lib" | grep -c '^qwen35$' || true)
  [ "$count" -gt 0 ] || {
    echo "bootstrap: $lib carries no qwen35 architecture, so this llama.cpp cannot serve the" >&2
    echo "           model. Upgrade llama.cpp and run this again." >&2
    exit 1; }
  did "libllama.dylib carries the qwen35 architecture ($count strings)"
}

step_checkout() {
  local before="" after
  if [ -d "$CHECKOUT/.git" ]; then
    before=$(git -C "$CHECKOUT" rev-parse HEAD)
    # --force: a tag the remote has moved is otherwise refused, and the run stops on the
    # first fetch after any rewrite of the repository's history.
    git -C "$CHECKOUT" fetch --force --tags --prune origin
    git -C "$CHECKOUT" checkout "$LOCALCODE_REF"
    # Only a branch can be fast-forwarded; a tag or a commit is already where it is going.
    if git -C "$CHECKOUT" symbolic-ref -q HEAD >/dev/null; then
      git -C "$CHECKOUT" pull --ff-only
    fi
    after=$(git -C "$CHECKOUT" rev-parse HEAD)
    if [ "$after" = "$before" ]; then
      skipped "$CHECKOUT is at $LOCALCODE_REF (${after:0:12})"
    else
      did "updated $CHECKOUT from ${before:0:12} to ${after:0:12}"
    fi
  else
    mkdir -p "$(dirname "$CHECKOUT")"
    git clone "$REPO" "$CHECKOUT"
    git -C "$CHECKOUT" checkout "$LOCALCODE_REF"
    after=$(git -C "$CHECKOUT" rev-parse HEAD)
    did "cloned $REPO into $CHECKOUT at $LOCALCODE_REF (${after:0:12})"
  fi
  HEAD_REV="${after:0:12}"
}

step_localcode() {
  # `localcode -version` names the revision it was built from, and `+modified` when that tree
  # was dirty — which never matches, so a modified checkout is always rebuilt. That is the
  # check: the checkout's path is compiled in, so nothing else here can tell a stale launcher
  # from a current one.
  local installed=""
  if [ -x "$HOME/.local/bin/localcode" ]; then
    installed=$("$HOME/.local/bin/localcode" -version 2>/dev/null | awk '{print $2}')
  fi
  if [ -n "$HEAD_REV" ] && [ "$installed" = "$HEAD_REV" ]; then
    skipped "localcode is the build of $HEAD_REV"
  else
    make -C "$CHECKOUT" install
    did "make install in $CHECKOUT (${installed:-none} to $HEAD_REV)"
  fi
  # The profile expands $HOME at login, not here: the line is the one README tells a reader
  # to add, and a path baked in now would be wrong for anyone else who ran this.
  # shellcheck disable=SC2016
  ensure_line "$HOME/.zprofile" 'export PATH="$HOME/.local/bin:$PATH"'
}

step_link() {
  # The link address itself is not applied here: it is a LaunchDaemon, because `ifconfig`
  # survives neither a reboot nor a replug. This only reads what the machine file says the
  # link is, so a missing value is found now rather than by an endpoint that never answers.
  local iface addr
  iface=$(machine_field link_interface)
  addr=$(machine_field link_address)
  skipped "the link daemon: $iface at $addr, installed by scripts/node/install.sh"
}

machine_field() { # key
  MACHINE_FILE="$CHECKOUT/$MACHINE" KEY="$1" python3 -c '
import json, os, sys
path, key = os.environ["MACHINE_FILE"], os.environ["KEY"]
try:
    print(json.load(open(path))[key])
except (OSError, ValueError, KeyError) as e:
    sys.exit(f"{path}: no {key} to read ({e})")'
}

step_command_line_tools
step_homebrew
step_packages
step_architecture_check
step_checkout
step_localcode
step_link

echo >&2
[ -z "$DID" ] || printf 'bootstrap: this run did:\n%s' "$DID" >&2
[ -z "$SKIPPED" ] || printf 'bootstrap: already in place:\n%s' "$SKIPPED" >&2
echo "bootstrap: next, from $CHECKOUT:" >&2
echo "  sudo LOCALCODE_NODE=1 SERVE_USER=$(id -un) ./scripts/node/prepare.sh" >&2
echo "  sudo LOCALCODE_NODE=1 SERVE_USER=$(id -un) ./scripts/node/install.sh" >&2
