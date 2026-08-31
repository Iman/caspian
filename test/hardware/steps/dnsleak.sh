# shellcheck shell=bash
#
# Step 5: the DNS leak check.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# While the phone browses, capture on the Pi's uplink and assert that no
# plaintext DNS leaves it.
#
# The method, and why it is not simply "count port 53 packets":
#
# Counting is confounded. The box resolves names itself, for the server address
# and for time, and design section 7 puts the box's own traffic outside the
# fail-closed guarantee deliberately. Worse, a client query that DID escape
# would be masqueraded on the way out and would look exactly like the box's own.
# A number with no attribution in it cannot answer the question.
#
# So the phone resolves a name that has never existed: a per-run random label
# under .invalid, which RFC 2606 reserves and which therefore reaches no
# nameserver anywhere and answers NXDOMAIN. Then one question is asked, and it
# has no attribution problem in it: did that label appear in cleartext on the
# uplink? Nothing else on the network can have produced it.
#
# It is resolved twice, on purpose:
#   - through Chrome, which is the real client, and
#   - through `ping`, which uses the OS resolver and so cannot be diverted by
#     Chrome's own Secure DNS.
# If Chrome has Secure DNS on and ping does not leak, the ping arm is what
# actually exercised the Pi's resolver path.
#
# WHAT THIS CHECK CANNOT SEE. This belongs in the output, not in a footnote:
#
#   DNS over HTTPS on port 443 is indistinguishable from any other HTTPS and is
#   carried through the tunnel like anything else. A client using it is INSIDE
#   the tunnel and invisible here. That is correct behaviour and not a failure.
#   The generated ruleset says the same thing in its own words, above the 853
#   rules in internal/netcfg/testdata/golden-ruleset-captured.nft:
#     "DNS over HTTPS on 443 is not distinguishable from other HTTPS and is
#      carried through the tunnel like anything else, which is a limit of this
#      design and not an oversight."
#
#   Also unseen: anything on an interface other than the uplink, and anything
#   outside the capture window.
#
# No packet capture leaves the Pi. The tcpdump output is consumed by awk on the
# box and two integers come back. See lib/pi.sh.

step_dnsleak() {
  local serial="$1" run="$2" window="${3:-30}"
  local dir probe watcher out packets hits url

  dir="$run/05-dns-leak"
  hw_ledger_start "$run" "dns-leak"
  mkdir -p "$dir"

  hw_step "DNS leak check on the Pi's uplink (${window}s window)"

  if ! pi_have; then
    hw_ledger_end "$run" "dns-leak" "SKIPPED"
    hw_say ""
    hw_say "RESULT NOT RUN: no PI_SSH in local/box.env, so the uplink cannot be watched."
    hw_say "  this is a step that did not happen. It is not a pass."
    return "$HW_PRECONDITION"
  fi

  if [ -n "${HOTSPOT_SSID:-}" ] && ! hw_is_dry; then
    ph_assert_ssid "$serial" "$HOTSPOT_SSID"
  fi

  # A label nobody has ever queried. Lowercase hex only, so it survives any
  # DNS 0x20 randomisation the resolver path might apply.
  probe="$(printf 'p%s%s.caspian-probe.invalid' "$(date -u '+%H%M%S')" "$$" | tr '[:upper:]' '[:lower:]')"
  hw_info "probe label for this run: $probe"

  out="$dir/uplink.txt"
  watcher="$(pi_dns_watch_start "$window" "$probe" "$out")"

  hw_is_dry || sleep 3

  hw_step "making the phone resolve the probe label, two ways"
  url="http://$probe/"
  hw_info "arm 1: Chrome navigates to $url"
  ph_chrome_open "$serial" "$url"
  hw_is_dry || sleep 6
  hw_info "arm 2: ping, which uses the OS resolver and not Chrome's"
  if hw_is_dry; then
    printf 'DRY  adb -s %s shell ping -c 2 -W 3 %s\n' "$serial" "$probe" >&2
  else
    adb -s "$serial" shell "ping -c 2 -W 3 $probe" >/dev/null 2>&1 || true
  fi
  hw_is_dry || sleep 4

  # A little real browsing alongside, so the window is not silent.
  # Traffic in the window, deliberately WITHOUT adding DNS to it. The echo
  # endpoint is a pinned IP literal, so this navigation resolves nothing: it
  # proves the window was not silent while leaving the only DNS in it the two
  # probe arms above, whose label is unique and therefore unambiguous.
  hw_info "and some traffic so the window is not silent (pinned IP, so it adds no DNS)"
  ph_chrome_open "$serial" "$EI_ECHO_A_URL?caspian=dns$(date -u '+%H%M%S')"

  if [ -n "$watcher" ] && [ "$watcher" != "$HW_DRY_SENTINEL" ] && ! hw_is_dry; then
    hw_info "waiting for the capture window to close"
    wait "$watcher" 2>/dev/null || true
  fi

  if hw_is_dry; then hw_dry_stop "$run" "dns-leak"; return $?; fi

  packets="$(sed -n 's/.*packets=\([0-9-]*\).*/\1/p' "$out" 2>/dev/null | head -1)"
  hits="$(sed -n 's/.*label_hits=\([0-9-]*\).*/\1/p' "$out" 2>/dev/null | head -1)"
  [ -n "$packets" ] || packets='-'
  [ -n "$hits" ] || hits='-'

  {
    printf 'window_seconds\t%s\n' "$window"
    printf 'plaintext_dns_packets_on_uplink\t%s\n' "$packets"
    printf 'probe_label_appearances\t%s\n' "$hits"
    printf 'limit\tDNS over HTTPS on 443 is inside the tunnel and invisible to this check\n'
    printf 'limit\tonly the uplink interface was watched, only during the window\n'
    printf 'limit\tno packets left the Pi; only these counts did\n'
  } | hw_write "$dir/result.tsv"

  hw_say ""
  if [ "$hits" = '-' ] || [ "$packets" = '-' ]; then
    hw_say "RESULT UNPROVEN: the capture on the Pi produced no counts."
    hw_say "  check that tcpdump is installed and that '$PI_SSH' can sudo without a password:"
    hw_say "    ssh $PI_SSH 'sudo -n tcpdump --version'"
    hw_ledger_end "$run" "dns-leak" "UNPROVEN"
    return "$HW_UNPROVEN"
  fi

  if [ "$hits" -gt 0 ] 2>/dev/null; then
    hw_say "RESULT LEAK: the probe label appeared in cleartext on the uplink $hits time(s)."
    hw_say "  a client DNS query escaped the tunnel. The label is unique to this run, so"
    hw_say "  nothing else on the network can have produced it and there is no attribution"
    hw_say "  question to argue about."
    hw_ledger_end "$run" "dns-leak" "LEAK"
    return "$HW_LEAK"
  fi

  hw_say "RESULT PASS: no plaintext client DNS escaped during the window."
  hw_say "  the probe label appeared 0 times on the uplink."
  hw_say "  $packets plaintext DNS-shaped packet(s) were seen on the uplink in total; those"
  hw_say "  are the box's own resolution, which design section 7 places outside the"
  hw_say "  guarantee on purpose. The probe label is what carries the verdict, not that count."
  hw_say ""
  hw_say "  WHAT THIS DID NOT CHECK, and cannot:"
  hw_say "    - DNS over HTTPS on port 443. It is inside the tunnel and indistinguishable"
  hw_say "      from other HTTPS. A client using it is not leaking; it is invisible here."
  hw_say "    - any interface other than the uplink."
  hw_say "    - anything outside the ${window}s window."
  hw_ledger_end "$run" "dns-leak" "OK"
  return "$HW_PASS"
}
