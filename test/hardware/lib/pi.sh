# shellcheck shell=bash
#
# Caspian-BYOC hardware harness: measurements taken on the Pi.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Two jobs, and one hard rule about what may cross back.
#
#   1. A third exit-IP source, independent of the phone and of Chrome, taken
#      through the engine's own diagnostics SOCKS listener. docs/LAYOUT.md's
#      port table puts 10808 on 127.0.0.1 and says what it is for in those
#      words: "SOCKS, for diagnostics and the exit-IP proof". This is the one
#      affordance the design already built for this harness.
#
#   2. The DNS check, which has to be taken on the uplink because that is the
#      only place a query that escaped is visible.
#
# THE RULE: no packet capture leaves the Pi. Not a pcap, not a hex dump, not a
# line of -A output. Every capture below is consumed by awk ON THE BOX and only
# integers come back. A capture on this uplink is a recording of the
# maintainer's own browsing, and there is no version of this harness that is
# worth keeping one.

pi_have() { [ -n "${PI_SSH:-}" ]; }

pi_require() {
  pi_have || hw_die "no PI_SSH is configured" \
    "put PI_SSH=user@host in local/box.env. Without it the uplink DNS check and the SOCKS cross-check cannot run, and a run without them is thinner than this project accepts."
}

pi_ssh() {
  if hw_is_dry; then
    printf 'DRY  ssh %s -- %s\n' "${PI_SSH:-<PI_SSH unset>}" "$*" >&2
    printf '%s\n' "$HW_DRY_SENTINEL"
    return 0
  fi
  # BatchMode so an unattended run fails fast instead of waiting on a password
  # prompt nobody is there to answer.
  ssh -o BatchMode=yes -o ConnectTimeout=10 "$PI_SSH" "$@"
}

# pi_uplink prints the uplink interface name.
#
# Read from the box rather than configured, because it changes: the design's
# section 9 records that the pinned host route is silently wrong after a DHCP
# renewal or a cable move, which is the same event that renames what "uplink"
# means. The MAIN table's default route is the uplink even while the tunnel is
# up, because the tunnel's default lives in table 8410
# (internal/netcfg/testdata/golden-commands-captured.txt: "ip route add default
# dev xray0 proto static table 8410").
pi_uplink() {
  if [ -n "${UPLINK_IFACE:-}" ]; then printf '%s\n' "$UPLINK_IFACE"; return 0; fi
  pi_ssh 'ip route show default | head -1' 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="dev") print $(i+1)}'
}

# pi_firewall_loaded reports whether the generated table is actually in force.
#
# This is what makes the fail-closed step mean something. "The phone reached
# nothing" is only a pass if the thing stopping it is the ruleset. If the table
# is absent the phone may be reaching nothing for a completely different
# reason, and the step must not be graded.
pi_firewall_loaded() {
  local out
  hw_is_dry && { printf 'not-checked-dry-run\n'; return 0; }
  out="$(pi_ssh 'sudo nft list table inet caspian >/dev/null 2>&1 && echo loaded || echo absent' 2>/dev/null | tr -d '\r')"
  # An unreachable box answers with nothing at all. Printing that blank would
  # read as "no firewall", which is a different and much more alarming fact.
  case "$out" in
    loaded|absent) printf '%s\n' "$out" ;;
    *)             printf 'unreachable\n' ;;
  esac
}

# pi_leak_rule_present looks for the single rule the whole ruleset exists for:
# the unconditional drop of hotspot-to-uplink forwarding. It is quoted in
# internal/netcfg/testdata/golden-ruleset-captured.nft as
#   iifname "ap0" oifname "eth0" drop comment "fail-closed: ..."
# and it names only the hotspot and the uplink, so the tunnel's absence cannot
# switch it off.
pi_leak_rule_present() {
  local hs up
  hs="${HOTSPOT_IFACE:-ap0}"
  up="$(pi_uplink)"
  [ -n "$up" ] || return 1
  pi_ssh "sudo nft list chain inet caspian forward 2>/dev/null" \
    | grep -q "iifname \"$hs\" oifname \"$up\" drop"
}

# pi_hotspot_iface prints the interface the box is ACTUALLY serving the hotspot
# on, discovered from the kernel, or nothing if no interface is.
#
# It exists because the rig had the name written down. local/box.env pinned
# HOTSPOT_IFACE=wlan0, which was true while the appliance took wlan0 over, and
# stopped being true on 2026-08-30 when it began creating ap0 instead. The
# hotspot was up and broadcasting and the harness reported "the Pi is not
# broadcasting Caspian-Wifi on wlan0", which is a rig fault wearing the costume
# of a product fault. A harness that asserts against a name somebody typed into
# a file months ago is asserting about the file.
#
# It selects on TYPE ONLY, deliberately. Selecting by SSID would make the
# "is it broadcasting the right name" check circular: it would find whatever is
# broadcasting the expected name and then confirm that it broadcasts it. So
# this answers "which interface is an access point", and the caller still has
# to check the name on whatever comes back.
#
# More than one access point interface is a FINDING, not a coin toss, so it
# reports them all and returns nothing rather than picking one.
pi_hotspot_iface() {
  local out names n
  out="$(pi_ssh "sudo iw dev" 2>/dev/null | tr -d '\r')"
  [ -n "$out" ] || return 1

  names="$(printf '%s\n' "$out" | awk '
    /^[[:space:]]*Interface[[:space:]]/ { iface = $2; next }
    /^[[:space:]]*type[[:space:]]+AP$/  { if (iface != "") print iface }
  ')"

  n="$(printf '%s' "$names" | grep -c . || true)"
  if [ "$n" -eq 0 ]; then
    return 1
  fi
  if [ "$n" -gt 1 ]; then
    hw_warn "more than one interface is an access point: $(printf '%s' "$names" | tr '\n' ' ')"
    hw_warn "refusing to guess which one is the hotspot"
    return 1
  fi
  printf '%s\n' "$names"
}

# pi_hotspot_addr prints the IPv4 address the Pi holds on the hotspot
# interface, discovered from the kernel, or nothing.
#
# Same lesson as pi_hotspot_iface, and found the same way. local/box.env carried
# HOTSPOT_ADDR=10.174.29.1 while the appliance was serving 10.83.51.1, so the
# fail-closed positive control asked the phone to fetch a page from an address
# that has not existed for some time, got nothing, and returned VOID on a box
# that was working. The identical request by hand answered "HTTP/1.0 303 See
# Other" immediately.
#
# The subnet is chosen by the appliance to avoid clashing with the network the
# box is already on, so it is not a constant and must never be written down.
pi_hotspot_addr() {
  local iface="$1" out
  [ -n "$iface" ] || return 1
  out="$(pi_ssh "ip -4 -br addr show dev ${iface}" 2>/dev/null | tr -d '\r')"
  printf '%s\n' "$out" | awk '{for (i=3; i<=NF; i++) if ($i ~ /^[0-9]+\./) {sub(/\/.*/, "", $i); print $i; exit}}'
}

# pi_hotspot_is_ap proves, from the kernel on the Pi, that the hotspot interface
# is actually an access point carrying the expected name.
#
# This is the check the rig did not have, and the reason it was needed: on
# 2026-08-30 the service logged "running hotspot=wlan0", the panel showed
# connected, hostapd was alive, and the interface was a station still joined to
# the house network broadcasting nothing. Everything the harness looked at was
# green and there was no hotspot. The December shell script this project learned
# from had this check (004-hotspot/xray-hotspot-fixed.sh, its verification step
# greps iw dev wlan0 info for "type AP"); ours dropped it.
#
# Measured on the target the same day, kernel 6.18.34 on brcmfmac, so the shape
# of the output is known rather than assumed:
#
#   with hostapd running      ssid Caspian-Probe / type AP / channel 6
#   typed AP, nothing serving  type AP / channel 10, and NO ssid line
#
# The second is why the name is checked and not only the mode: an interface can
# be an access point and be broadcasting nothing at all.
pi_hotspot_is_ap() {
  local iface="$1" want_ssid="${2:-}" out mode ssid
  out="$(pi_ssh "sudo iw dev ${iface} info" 2>/dev/null | tr -d '\r')"
  if [ -z "$out" ]; then
    hw_warn "could not read ${iface} back from the Pi"
    return 1
  fi
  mode="$(printf '%s\n' "$out" | sed -n 's/^[[:space:]]*type[[:space:]]*//p' | head -1)"
  ssid="$(printf '%s\n' "$out" | sed -n 's/^[[:space:]]*ssid[[:space:]]*//p' | head -1)"

  if [ "$mode" != "AP" ]; then
    hw_warn "${iface} reports type '${mode}', not AP: there is no hotspot on it"
    return 1
  fi
  if [ -z "$ssid" ]; then
    hw_warn "${iface} is an access point and is broadcasting no name"
    return 1
  fi
  if [ -n "$want_ssid" ] && [ "$ssid" != "$want_ssid" ]; then
    hw_warn "${iface} is broadcasting '${ssid}', not '${want_ssid}'"
    return 1
  fi
  hw_info "${iface} is an access point broadcasting '${ssid}'"
  return 0
}

# pi_socks_exit prints the exit IP as seen through the engine's own SOCKS
# listener. Independent of the phone entirely.
#
# What it proves: the tunnel is up and egressing from a particular address
# right now. What it does NOT prove, and this matters: that the PHONE's traffic
# used it. Those are different facts and this harness never lets the second be
# inferred from the first.
# The body it fetches is a cdn-cgi/trace, so it is read with ei_read_a for the
# same reason the phone's copy is: the first IP literal in that body is the
# endpoint's own constant address, not an exit address.
pi_socks_exit() {
  local url="$1" port
  port="${CASPIAN_HW_SOCKS_PORT:-10808}"
  pi_ssh "curl -fsS --max-time 20 --socks5-hostname 127.0.0.1:$port '$url'" 2>/dev/null
}

# ---------------------------------------------------------------------------
# The DNS check.
#
# pi_dns_watch runs a capture for N seconds and prints two integers:
#
#   packets=<n>      plaintext DNS-shaped packets seen leaving on the uplink
#   label_hits=<n>   how many of them contained the unique probe label
#
# The unique label is what makes this precise. Counting plaintext DNS on the
# uplink alone is confounded: the BOX itself resolves names, for the server
# address and for time, and design section 7 puts the box's own traffic outside
# the fail-closed guarantee on purpose. After masquerade a client's leaked
# query would look like the box's own. So the harness has the PHONE resolve a
# name that has never existed anywhere, and asks one question with no
# attribution problem in it: did that label appear in cleartext on the uplink?
#
#   label_hits > 0   a client query escaped the tunnel. That is the finding.
#   label_hits = 0   no client query escaped in cleartext during the window.
#
# What this cannot see, and it belongs in the output rather than in a footnote:
#
#   - DNS over HTTPS on port 443. Indistinguishable from any other HTTPS, and
#     carried through the tunnel like anything else. The generated ruleset says
#     so itself in golden-ruleset-captured.nft, above the 853 rules. If Chrome
#     has its own Secure DNS switched on, the query never reaches the Pi's
#     resolver and never appears here, and that is CORRECT behaviour, not a
#     pass and not a failure. It is outside the check.
#   - Anything on an interface other than the uplink.
#   - Anything outside the capture window.
#
# 853 is included in the filter although the ruleset rejects DoT and drops DoQ,
# because a rule that is supposed to be there is exactly the thing worth
# measuring rather than assuming.
# ---------------------------------------------------------------------------
pi_dns_watch_cmd() {
  local secs="$1" label="$2" iface="$3"
  printf '%s' "sudo timeout $secs tcpdump -n -l -i $iface -s 512 -A 'port 53 or port 853 or udp port 5353' 2>/dev/null | awk -v lbl='$label' 'BEGIN{p=0;h=0} /^[0-9][0-9]:[0-9][0-9]:[0-9][0-9]/{p++} index(\$0,lbl)>0{h++} END{printf \"packets=%d label_hits=%d\\n\",p,h}'"
}

# pi_dns_watch_start launches the capture in the background on the Mac side and
# writes the two integers to a file when it finishes.
pi_dns_watch_start() {
  local secs="$1" label="$2" out="$3" iface cmd
  iface="$(pi_uplink)"
  if [ -z "$iface" ] || [ "$iface" = "$HW_DRY_SENTINEL" ]; then iface='<uplink>'; fi
  cmd="$(pi_dns_watch_cmd "$secs" "$label" "$iface")"
  if hw_is_dry; then
    printf 'DRY  would run on the Pi for %ss:\n' "$secs" >&2
    printf 'DRY    %s\n' "$cmd" >&2
    printf 'packets=0 label_hits=0\n' > "$out"
    return 0
  fi
  hw_info "capturing on the Pi uplink ($iface) for ${secs}s"
  ( pi_ssh "$cmd" > "$out" 2>/dev/null || printf 'packets=- label_hits=-\n' > "$out" ) &
  printf '%s\n' "$!"
}
