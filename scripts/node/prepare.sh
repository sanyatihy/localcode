#!/usr/bin/env bash
# Prepare a dedicated serving node: apply every OS lever 0056 lists, then read what macOS
# still keeps when nothing is running and check it against the node's record.
#
# The node's budget is total memory less what macOS keeps idle, and both are readings. A
# lever here exists because it costs memory or wakes the machine, not because it is a
# hardening habit; what each one saves is measured on its own and recorded in docs/TECH.md,
# and a lever that saves under 50 MB is dropped from this script.
#
# Wi-Fi is deliberately untouched: the node joins from the system network profile at the
# login window, and that is how it is reachable when the Thunderbolt bridge is not.
#
#   sudo LOCALCODE_NODE=1 SERVE_USER=<user> ./scripts/node/prepare.sh
set -euo pipefail
cd "$(dirname "$0")/../.."

MACHINE="${MACHINE:-config/machine-m5pro-24gb.json}"
SERVER_BIN="${SERVER_BIN:-llama-server}"

# Every lever below turns off something a person at this machine would want, and several of
# them are not undone by a reboot. A node is the only machine they belong on.
[ "${LOCALCODE_NODE:-}" = "1" ] || {
  echo "prepare: this turns off Spotlight, Bluetooth, sleep, sharing and the update daemon," >&2
  echo "         which is a dedicated node's configuration and ruins a machine somebody works" >&2
  echo "         at. Set LOCALCODE_NODE=1 on the node to confirm that is what this machine is." >&2
  exit 2; }
[ "$(id -u)" = "0" ] || { echo "prepare: most of these levers need root; run under sudo" >&2; exit 2; }

SERVE_USER="${SERVE_USER:-${SUDO_USER:-}}"
[ -n "$SERVE_USER" ] || { echo "prepare: name the serving user in SERVE_USER" >&2; exit 2; }
SERVE_HOME=$(dscl . -read "/Users/$SERVE_USER" NFSHomeDirectory 2>/dev/null | awk '{print $2}')
[ -n "$SERVE_HOME" ] || { echo "prepare: no such user: $SERVE_USER" >&2; exit 2; }

# What one run could not apply, collected rather than fatal: a node is prepared by reading
# the whole list once, not by rerunning this after each refusal. It is still a failure.
NEEDS_HUMAN=""
needs_human() { NEEDS_HUMAN="${NEEDS_HUMAN}  - ${1}
"; echo "prepare: $1" >&2; }

# A per-user preference belongs to the serving user even when root applies it: at the login
# window there is nobody else, and root's own domain is read by nothing that matters here.
as_user() { sudo -u "$SERVE_USER" "$@"; }

# One refused lever must not hide the rest, so the failure is named and the walk continues.
try() { "$@" >/dev/null 2>&1 || needs_human "refused: $*"; }

# Remote Login and the group that restricts it need the calling process to hold Full Disk
# Access, which being root does not confer: the grant is on the terminal or SSH client that
# ran this. Their stderr goes through rather than into /dev/null, because "operation not
# permitted" from one of these means that grant and nothing else.
needs_fda() {
  "$@" || needs_human "$* failed — give the terminal or SSH client that ran this Full Disk Access in System Settings > Privacy & Security, then run it again"
}

step() { echo "prepare: $1" >&2; }

# bootout_now stops a service that is running now. launchctl answers 3 for one that was not
# loaded, which is the state this wants rather than a failure.
bootout_now() {
  local label="$1" out status=0
  out=$(launchctl bootout "system/$label" 2>&1) || status=$?
  if [ "$status" -ne 0 ] && [ "$status" -ne 3 ]; then
    needs_human "could not stop $label (launchctl exit $status): $out"
  fi
}

# Logged out at the login window is a precondition rather than a lever: a session holds the
# compositor, the Dock and whatever the user left open, and the idle reading below would
# measure those instead of macOS.
precondition_logged_out() {
  local console
  console=$(who | awk '$2 == "console" { print $1 }')
  [ -z "$console" ] || {
    echo "prepare: $console is logged in at the console; log out to the login window first —" >&2
    echo "         the idle reading is what macOS keeps with nobody logged in" >&2
    exit 2; }
}

lever_apple_intelligence() {
  # unverified on the node
  step "Apple Intelligence off"
  try as_user defaults write com.apple.CloudSubscriptionFeatures.optIn device -bool false
}

lever_siri() {
  # unverified on the node
  step "Siri off"
  try as_user defaults write com.apple.assistant.support "Assistant Enabled" -bool false
  try as_user defaults write com.apple.Siri StatusMenuVisible -bool false
  try as_user defaults write com.apple.Siri VoiceTriggerUserEnabled -bool false
}

lever_spotlight() {
  step "Spotlight indexing off"
  # Indexing a 17 GB weights download is work nobody asked for, and it runs while the model
  # serves. The volumes stay indexable; the indexer stops.
  try mdutil -a -i off
}

lever_icloud() {
  # unverified on the node
  step "iCloud signed out"
  # No supported command signs a Mac out of iCloud, so this reads whether it is signed in and
  # says so. A node signed into an account syncs and phones home on its own schedule.
  local accounts
  accounts=$(as_user defaults read MobileMeAccounts Accounts 2>/dev/null || true)
  [ -z "$accounts" ] || needs_human "$SERVE_USER is signed into iCloud; sign out in System Settings, which is the only way"
}

lever_handoff() {
  # unverified on the node
  step "Handoff off"
  try as_user defaults -currentHost write com.apple.coreservices.useractivityd ActivityAdvertisingAllowed -bool false
  try as_user defaults -currentHost write com.apple.coreservices.useractivityd ActivityReceivingAllowed -bool false
}

lever_airplay() {
  # unverified on the node
  step "AirPlay receiver off"
  # Host-scoped: the receiver is a per-machine setting under the user's ByHost preferences,
  # and a write without -currentHost lands where nothing reads it. Both keys, because the
  # NIST macOS baseline names the second for Sequoia and the first is what older releases read.
  try as_user defaults -currentHost write com.apple.controlcenter AirplayRecieverEnabled -bool false
  try as_user defaults -currentHost write com.apple.controlcenter system_settings_airplay_receiver_disable -bool true
}

lever_sharing() {
  # unverified on the node
  step "every sharing service but Remote Login off"
  # disable survives a reboot; bootout stops what is running now. Remote Login is not here:
  # it is the one service this node answers on, and the next lever turns it on.
  local service
  for service in com.apple.screensharing com.apple.smbd com.apple.AppleFileServer \
                 com.apple.RemoteDesktop.PrivilegeProxy; do
    try launchctl disable "system/$service"
    launchctl bootout "system/$service" >/dev/null 2>&1 || true
  done
  try /System/Library/CoreServices/RemoteManagement/ARDAgent.app/Contents/Resources/kickstart -deactivate -stop
  try systemsetup -setremoteappleevents off
  try cupsctl --no-share-printers
  try AssetCacheManagerUtil deactivate
  try defaults write /Library/Preferences/SystemConfiguration/com.apple.nat NAT -dict Enabled -int 0
}

lever_time_machine() {
  step "Time Machine off"
  # A backup pass reads the whole disk and wires its own buffers, during a run nobody is
  # watching. The node holds no state worth backing up: the weights are a download.
  try tmutil disable
}

lever_software_update() {
  # unverified on the node
  step "automatic updates and softwareupdated off"
  try softwareupdate --schedule off
  try defaults write /Library/Preferences/com.apple.SoftwareUpdate AutomaticCheckEnabled -bool false
  try defaults write /Library/Preferences/com.apple.SoftwareUpdate AutomaticDownload -bool false
  try defaults write /Library/Preferences/com.apple.SoftwareUpdate AutomaticallyInstallMacOSUpdates -bool false
  try defaults write /Library/Preferences/com.apple.SoftwareUpdate CriticalUpdateInstall -bool false
  try defaults write /Library/Preferences/com.apple.commerce AutoUpdate -bool false
  # The daemon as well as the schedule: an update that downloads while the model serves is
  # memory and bandwidth nobody budgeted, and a reboot it chose is a run lost. disable keeps
  # it from starting again and does nothing to the instance already running, so it is booted
  # out too — the idle reading below is taken with it gone.
  try launchctl disable system/com.apple.softwareupdated
  bootout_now com.apple.softwareupdated
}

lever_bluetooth() {
  # unverified on the node
  step "Bluetooth off"
  try defaults write /Library/Preferences/com.apple.Bluetooth ControllerPowerState -int 0
  launchctl kickstart -k system/com.apple.bluetoothd >/dev/null 2>&1 || true
}

lever_screen_saver() {
  # unverified on the node
  step "login window screen saver off"
  # The node sits at the login window with nobody logged in, so the serving user's own
  # idleTime is never consulted: the login window reads its own. Either way there is no
  # display attached, and the saver only ever wakes the GPU to draw what nobody sees.
  try defaults write /Library/Preferences/com.apple.screensaver loginWindowIdleTime -int 0
}

lever_power() {
  step "sleep disabled on power, restart after a power failure"
  # disablesleep is what keeps a closed lid from sleeping the node. autorestart is what
  # makes a power cut end in a serving node rather than a dark one.
  try pmset -a disablesleep 1
  try pmset -c sleep 0 autorestart 1
}

lever_remote_login() {
  # unverified on the node
  step "Remote Login on, $SERVE_USER only, keys only"
  local ssh_group=com.apple.access_ssh info member
  # Order matters: a node whose serving user has no key installed and whose password
  # authentication is off is a node nobody can log into, and it has no screen.
  [ -s "$SERVE_HOME/.ssh/authorized_keys" ] || {
    echo "prepare: $SERVE_HOME/.ssh/authorized_keys is empty or absent, and turning password" >&2
    echo "         authentication off with no key installed locks this node out for good" >&2
    exit 2; }
  # -f because systemsetup asks for confirmation otherwise, and there is nobody to ask.
  needs_fda systemsetup -f -setremotelogin on
  # com.apple.access_ssh is the group sshd restricts logins to, and it admits whoever is
  # already in it — an account or a nested group left there from before is a way into a node
  # that answers nothing else. So membership is set to exactly the serving user, not added to.
  dseditgroup -o read "$ssh_group" >/dev/null 2>&1 || needs_fda dseditgroup -o create -q "$ssh_group"
  info=$(dscl . -read "/Groups/$ssh_group" GroupMembership NestedGroups 2>/dev/null || true)
  while IFS= read -r member; do
    { [ -n "$member" ] && [ "$member" != "$SERVE_USER" ]; } || continue
    needs_fda dseditgroup -o edit -d "$member" -t user "$ssh_group"
  done < <(printf '%s\n' "$info" | awk '/^GroupMembership:/ { for (i = 2; i <= NF; i++) print $i }')
  while IFS= read -r member; do
    [ -n "$member" ] || continue
    needs_fda dseditgroup -o edit -d "$member" -t group "$ssh_group"
  done < <(printf '%s\n' "$info" | awk '/^NestedGroups:/ { for (i = 2; i <= NF; i++) print $i }')
  needs_fda dseditgroup -o edit -a "$SERVE_USER" -t user "$ssh_group"
  # A drop-in rather than an edit: an OS update replaces /etc/ssh/sshd_config and would take
  # the setting with it.
  if grep -q '^Include /etc/ssh/sshd_config.d/' /etc/ssh/sshd_config; then
    printf '%s\n' "# Written by scripts/node/prepare.sh (0056)." \
      "PasswordAuthentication no" "KbdInteractiveAuthentication no" "PermitRootLogin no" \
      > /etc/ssh/sshd_config.d/100-localcode-node.conf
    chmod 644 /etc/ssh/sshd_config.d/100-localcode-node.conf
  else
    needs_human "/etc/ssh/sshd_config includes no sshd_config.d, so password authentication has to be turned off in it by hand"
  fi
}

lever_firewall() {
  # unverified on the node
  step "application firewall on, stealth, sshd and $SERVER_BIN only"
  local fw=/usr/libexec/ApplicationFirewall/socketfilterfw app server existing apps
  try "$fw" --setglobalstate on
  try "$fw" --setstealthmode on
  # Block-all admits nothing at all, including the server, so a node left in that state
  # answers no request and looks like a dead endpoint. The allowance list is the boundary.
  try "$fw" --setblockall off
  # Without these two every signed binary is admitted, which is not "sshd and the server".
  try "$fw" --setallowsigned off
  try "$fw" --setallowsignedapp off
  # The list is set rather than added to: an allowance somebody granted once is a way in
  # that no lever here turns off, and "only these" is the claim this lever makes.
  apps=$("$fw" --listapps 2>/dev/null | sed -n 's/^[0-9][0-9]* *: *\(\/.*\)$/\1/p' || true)
  while IFS= read -r existing; do
    [ -n "$existing" ] || continue
    try "$fw" --remove "$existing"
  done < <(printf '%s\n' "$apps")
  server=$(command -v "$SERVER_BIN" || true)
  [ -n "$server" ] || needs_human "$SERVER_BIN is not on PATH, so the firewall has no server binary to admit"
  # macOS launches SSH through sshd-keygen-wrapper, so a list holding /usr/sbin/sshd alone
  # can refuse the next connection to a node whose only way in is SSH.
  for app in /usr/sbin/sshd /usr/libexec/sshd-keygen-wrapper "$server"; do
    [ -n "$app" ] || continue
    try "$fw" --add "$app"
    try "$fw" --unblockapp "$app"
  done
}

# What macOS keeps with nobody logged in and nothing serving, against what this node read
# when it was prepared. Above the record means something is running that these levers do not
# turn off, and the node's reserve — total less this — was computed on the smaller number.
idle_reading() {
  local reading
  reading=$(./scripts/memprobe.sh)
  READING="$reading" MACHINE_FILE="$MACHINE" python3 -c '
import json, os, sys

r = json.loads(os.environ["READING"])
path = os.environ["MACHINE_FILE"]
m = json.load(open(path))
anon, wired = r["anonymous_gb"], r["wired_gb"]
record_a, record_w = m.get("idle_anonymous_gb"), m.get("idle_wired_gb")
if record_a is None or record_w is None:
    print(f"idle: anonymous {anon} GB, wired {wired} GB")
    print(f"idle: {path} records no idle reading yet, so there is nothing to check these against")
    sys.exit(0)
print(f"idle: anonymous {anon} GB against a recorded {record_a}, wired {wired} GB against {record_w}")
sys.stdout.flush()
if anon > record_a or wired > record_w:
    sys.exit("idle: this node reads above its own record, so it is not prepared")
'
}

precondition_logged_out
lever_apple_intelligence
lever_siri
lever_spotlight
lever_icloud
lever_handoff
lever_airplay
lever_sharing
lever_time_machine
lever_software_update
lever_bluetooth
lever_screen_saver
lever_power
lever_remote_login
lever_firewall

prepared=0
idle_reading || prepared=1
if [ -n "$NEEDS_HUMAN" ]; then
  prepared=1
  printf 'prepare: these did not apply:\n%s' "$NEEDS_HUMAN" >&2
fi
[ "$prepared" = 0 ] || exit 1
echo "prepare: every lever applied; close the lid with no display attached" >&2
