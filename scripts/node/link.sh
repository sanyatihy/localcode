#!/usr/bin/env bash
# Hold the node's end of the USB link to the laptop. Run at boot and every 30 s by
# com.localcode.link.
#
# It reconciles rather than applies once: `ifconfig` loses the address on a replug as well as
# on a reboot, and a job that ran at load has no way to notice. Nothing is logged when the
# address is already there, because this runs twice a minute forever.
set -euo pipefail
cd "$(dirname "$0")/../.."

MACHINE="${MACHINE:-config/machine-m5max-36gb.json}"
NETMASK="${NETMASK:-255.255.255.0}"

# Assigned first, then split: a here-string from a failed substitution still reads as one
# empty line, which would take an unset interface to ifconfig instead of stopping here.
link=$(MACHINE_FILE="$MACHINE" python3 -c '
import json, os, sys
path = os.environ["MACHINE_FILE"]
try:
    m = json.load(open(path))
    print(m["link_interface"], m["link_address"])
except (OSError, ValueError, KeyError) as e:
    sys.exit(f"{path}: no link to read ({e})")')
read -r iface addr <<<"$link"

# The cable is out: there is no interface to configure and nothing to report. Saying so
# twice a minute would fill the log with the normal state of an unplugged machine.
ifconfig "$iface" >/dev/null 2>&1 || exit 0

ifconfig "$iface" | grep -qF "inet $addr " && exit 0

ifconfig "$iface" inet "$addr" netmask "$NETMASK"
echo "link: $iface set to $addr" >&2
