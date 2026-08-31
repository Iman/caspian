# shellcheck shell=bash
#
# Step 4: fail-closed.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Engine stopped, hotspot still up, phone still joined. The phone must reach
# nothing. Traffic that still flows is the most serious result this harness can
# produce and it must be impossible to mistake for a pass.
#
# Three things make this step trustworthy rather than vacuous, and all three are
# checked before any traffic is driven:
#
#   1. A POSITIVE CONTROL. "The phone reached nothing" is worthless if the
#      hotspot fell over: a phone with no link reaches nothing too, and that
#      would read as a pass. So the phone must still reach the PANEL on the Pi
#      while reaching nothing beyond it. The ruleset already permits exactly
#      that and nothing else: golden-ruleset-captured.nft's input chain has
#        iifname "ap0" tcp dport 8088 accept comment "panel"
#      so a reachable panel plus an unreachable internet is the precise shape of
#      a working fail-closed, and it is distinguishable from a dead hotspot.
#
#   2. NO CELLULAR. When the hotspot stops reaching the internet Android marks
#      it unvalidated and moves the default route to mobile data. The phone then
#      reaches everything, over LTE, and a careless harness calls that a leak
#      when the packets never went near the Pi. Measured on the attached
#      handset: it has a SIM and mobile data was on. --wifi-only uses airplane
#      mode with the wifi radio turned back on, which was measured working from
#      the shell uid on 2026-08-30 and restored afterwards.
#
#   3. THE RULESET IS ACTUALLY LOADED. If the privileged service died and took
#      its nftables table with it, the phone might reach nothing for an
#      unrelated reason, or reach everything. Either way the step is not about
#      the firewall any more. Checked over ssh when the Pi is reachable, and
#      reported as an unchecked assumption when it is not.

step_failclosed() {
  local serial="$1" run="$2" label="$3" wifi_only="${4:-0}"
  local dir baseline fw panel_body panel_ok line a b kind_a kind_b void restored=0 hs_iface hs_addr lease

  dir="$run/04-fail-closed"
  hw_ledger_start "$run" "fail-closed"

  hw_step "fail-closed: engine stopped, hotspot up, phone must reach nothing"
  hw_info "the config that was running: $label"
  baseline="$(step_baseline_load "$run")"

  if [ -z "${HOTSPOT_ADDR:-}" ]; then
    hw_ledger_end "$run" "fail-closed" "PRECONDITION"
    hw_die "HOTSPOT_ADDR is not set in local/box.env" \
      "set it to the Pi's address on the hotspot interface, for example HOTSPOT_ADDR=10.83.51.1. Without it there is no positive control, and 'reached nothing' cannot be told apart from 'the hotspot is down'."
  fi

  # Bring the box UP first. This step's whole shape is "up, then stop only the
  # engine", so it needs something running to stop. Inside "all" the preceding
  # prove step leaves the box up and this was invisible; run on its own, the
  # install that precedes it restarts the appliance and leaves it off, and the
  # step then reported "there is no hotspot" as a product failure when the
  # truth is that nobody had switched it on.
  ctl_apply "$label"

  # Prove the hotspot exists and put the phone back on it BEFORE anything is
  # asked of it. This step used to assume the phone was already joined, which
  # is true when it runs inside "all" and false whenever it is run on its own:
  # the install that precedes it restarts the appliance, the hotspot goes down,
  # and the phone falls back to the house network. It then reported
  # "the phone is on wifi HomeNet" as a product failure.
  if pi_have && ! hw_is_dry; then
    hs_iface="$(pi_hotspot_iface || true)"
    if [ -z "$hs_iface" ]; then
      hw_ledger_end "$run" "fail-closed" "PRECONDITION"
      hw_die "no interface on the Pi is an access point, so there is no hotspot to test against" \
        "nothing was asked of the phone. The box has to be up before fail-closed means anything."
    fi
    if ! pi_hotspot_is_ap "$hs_iface" "$HOTSPOT_SSID"; then
      hw_ledger_end "$run" "fail-closed" "PRECONDITION"
      hw_die "the Pi is not broadcasting '$HOTSPOT_SSID' on $hs_iface" \
        "nothing was asked of the phone."
    fi
    # The address is discovered too, for the same reason the interface is.
    # A configured one is honoured only if the box agrees with it.
    hs_addr="$(pi_hotspot_addr "$hs_iface" || true)"
    if [ -n "$hs_addr" ]; then
      if [ -n "${HOTSPOT_ADDR:-}" ] && [ "$HOTSPOT_ADDR" != "$hs_addr" ]; then
        hw_info "the hotspot address is ${hs_addr}, not the configured ${HOTSPOT_ADDR}; using what the box is doing"
      fi
      HOTSPOT_ADDR="$hs_addr"
    fi
    hw_info "the panel on the hotspot is at http://${HOTSPOT_ADDR}:${PANEL_PORT:-8088}/"

    ph_join "$serial" "$HOTSPOT_SSID"
    case $? in
      0|2) : ;;
      *)
        hw_ledger_end "$run" "fail-closed" "PRECONDITION"
        hw_die "could not put the phone on '$HOTSPOT_SSID'" \
          "this is a rig problem, not a product result."
        ;;
    esac
  fi

  if [ "$wifi_only" = "1" ]; then
    ph_wifi_only_enter "$serial"
    restored=1
  fi
  if ! hw_is_dry; then
    ph_assert_no_cellular "$serial"
    [ -n "${HOTSPOT_SSID:-}" ] && ph_assert_ssid "$serial" "$HOTSPOT_SSID"
  fi

  hw_step "checking the firewall is the thing doing the blocking"
  if pi_have; then
    fw="$(pi_firewall_loaded)"
    hw_info "nftables table inet caspian: $fw"
    if [ "$fw" = "absent" ]; then
      [ "$restored" = "1" ] && ph_wifi_only_leave "$serial"
      hw_ledger_end "$run" "fail-closed" "PRECONDITION"
      hw_die "the generated nftables table is not loaded on the Pi" \
        "whatever this step measured would not be about fail-closed. Start the privileged service and re-run."
    fi
    if hw_is_dry; then
      hw_info "would check the forward chain for the hotspot-to-uplink drop rule"
    elif pi_leak_rule_present; then
      hw_info "the hotspot-to-uplink drop rule is present"
    else
      hw_warn "could not find the 'iifname <hotspot> oifname <uplink> drop' rule in the forward chain."
      hw_warn "the step will still run, but a pass would then rest on the chain policy alone."
    fi
  else
    hw_warn "no PI_SSH configured, so THE FIREWALL STATE WAS NOT CHECKED."
    hw_warn "a pass below means 'the phone reached nothing', not 'the ruleset stopped it'."
  fi

  ctl_stop
  hw_is_dry || sleep 5

  # Wait for the phone to be BACK ON the hotspot with an address before asking
  # it anything, rather than sleeping a fixed five seconds and hoping.
  #
  # This step toggles airplane mode to force wifi-only, which drops the
  # association. Re-associating and taking a DHCP lease takes longer than the
  # sleep did, so the positive control ran against a phone that was not on the
  # network yet, could not reach the panel, and the whole step returned VOID.
  # It did that twice on 2026-08-30, both times on an appliance that was
  # working: proved by hand immediately afterwards, panel answering 303 and
  # four independent internet targets on three ports all returning nothing.
  #
  # A rig that reports VOID on a working box teaches people to ignore VOID.
  if ! hw_is_dry; then
    hw_step "waiting for the phone to hold an address on the hotspot again"
    lease=""
    for _ in $(seq 1 30); do
      lease="$(ph_sh "$serial" "ip -f inet addr show wlan0 2>/dev/null | sed -n 's/.*inet \([0-9.]*\).*/\1/p' | head -1" | tr -d '\r')"
      case "$lease" in
        "" ) : ;;
        169.254.* ) lease="" ;;   # link-local is no lease at all
        * ) break ;;
      esac
      sleep 2
    done
    if [ -z "$lease" ]; then
      [ "$restored" = "1" ] && ph_wifi_only_leave "$serial"
      hw_ledger_end "$run" "fail-closed" "VOID"
      hw_die "the phone never took an address on the hotspot after the airplane-mode toggle" \
        "so nothing it failed to reach would mean anything. This is a rig timing problem, not a product result."
    fi
    hw_info "the phone holds ${lease} on the hotspot"
  fi

  # ---- positive control ----
  hw_step "positive control: the phone must still reach the panel on the Pi"
  panel_body="$(ph_http_get "$serial" "$HOTSPOT_ADDR" "/login" "${PANEL_PORT:-8088}")"
  printf '%s\n' "$panel_body" | hw_write "$dir/panel-reachable.txt"
  panel_ok=0
  printf '%s' "$panel_body" | grep -qE 'HTTP/1\.[01] [0-9]{3}' && panel_ok=1
  if hw_is_dry; then panel_ok=1; fi

  if [ "$panel_ok" != "1" ]; then
    [ "$restored" = "1" ] && ph_wifi_only_leave "$serial"
    hw_ledger_end "$run" "fail-closed" "VOID"
    hw_say ""
    hw_say "RESULT VOID: the phone could not reach the panel either."
    hw_say "  the hotspot is not carrying traffic at all, so 'reached nothing' says nothing"
    hw_say "  about the firewall. Bring the hotspot back and retake this."
    hw_say "  check: adb -s $serial shell cmd wifi status"
    hw_say "  check: on the Pi, systemctl status caspian.service"
    return "$HW_VOID"
  fi
  hw_info "the panel answered, so the hotspot and its L3 path are alive"

  # ---- the actual test ----
  hw_step "driving traffic that must not arrive"
  ph_cdp_up "$serial"
  line="$(ei_capture "$serial" "$dir")"
  ph_cdp_down "$serial"
  hw_info "capture: $line"

  if hw_is_dry; then
    ph_wifi_only_leave "$serial"
    hw_dry_stop "$run" "fail-closed"; return $?
  fi

  a="$(ei_field a "$line")"; b="$(ei_field b "$line")"
  kind_a="$(ei_field a_kind "$line")"; kind_b="$(ei_field b_kind "$line")"
  void="$(ei_field void "$line")"

  [ "$restored" = "1" ] && ph_wifi_only_leave "$serial"

  {
    printf 'config_that_was_running\t%s\n' "$label"
    printf 'panel_reachable\tyes\n'
    printf 'source_a\t%s\n' "$kind_a"
    printf 'source_b\t%s\n' "$kind_b"
    printf 'traffic_escaped\t%s\n' "$( { [ "$a" != '-' ] || [ "$b" != '-' ]; } && printf 'yes' || printf 'no' )"
  } | hw_write "$dir/result.tsv"

  if [ "$void" = "1" ]; then
    hw_ledger_end "$run" "fail-closed" "VOID"
    hw_say ""
    hw_say "RESULT VOID: the phone changed network state during the capture."
    hw_say "  retake it. Not a pass, not a leak."
    return "$HW_VOID"
  fi

  hw_say ""
  if [ "$a" != '-' ] || [ "$b" != '-' ]; then
    hw_say "############################################################"
    hw_say "RESULT LEAK. The engine was stopped and traffic still left the box."
    hw_say "############################################################"
    hw_say "  source A (Chrome): $( [ "$a" != '-' ] && printf 'reached the internet and was answered by %s' "$a" || printf 'was blocked' )"
    hw_say "  source B (nc):     $( [ "$b" != '-' ] && printf 'reached the internet and was answered by %s' "$b" || printf 'was blocked' )"
    if [ "$a" = "$baseline" ] || [ "$b" = "$baseline" ]; then
      hw_say "  the address is the phone's untunnelled baseline, so this is the uplink path"
      hw_say "  exactly as design section 7 predicts when a rule is missing."
    fi
    hw_say "  this is the worst result the harness can produce. Nothing else in this run"
    hw_say "  matters until it is fixed."
    hw_ledger_end "$run" "fail-closed" "LEAK"
    return "$HW_LEAK"
  fi

  hw_say "RESULT PASS: with the engine stopped, the phone reached nothing."
  hw_say "  source A (Chrome): $kind_a"
  hw_say "  source B (nc):     $kind_b"
  hw_say "  and the panel was still reachable throughout, so the hotspot was up and this"
  hw_say "  is the firewall refusing traffic rather than a dead link."
  if [ "$wifi_only" != "1" ]; then
    hw_say "  note: mobile data was not forced off by the harness, only asserted."
  fi
  hw_ledger_end "$run" "fail-closed" "OK"
  return "$HW_PASS"
}
