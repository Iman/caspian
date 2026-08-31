# shellcheck shell=bash
#
# Step 2: prove one config.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Bring the box up with one config, join the phone to the hotspot, push real
# traffic, capture the exit IP from two independent sources, and match it
# against the server the config names.
#
# The three outcomes the report may carry, and nothing else:
#   PASS      the box NAME, because a matching exit IP IS the server address and
#             printing it would put a config's server address in a log
#   FAIL      the observed address, when it matches no configured box, because
#             then it is a measurement and not a config's secret. When it
#             matches a DIFFERENT config, the other box's NAME is printed for
#             the same reason as above
#   UNPROVEN  no exit IP was captured. Never written as a pass, never softened

step_prove() {
  local serial="$1" run="$2" label="$3"
  local dir baseline line a b kind_a kind_b void agree ip verdict desc socks fp classified

  dir="$run/02-prove-$label"
  hw_ledger_start "$run" "prove:$label"

  cfg_require "$label"
  cfg_assert_supported "$label" >/dev/null
  hw_step "proving config '$label' ($(cfg_scheme "$label"))"
  cfg_crosscheck "$label"

  baseline="$(step_baseline_load "$run")"

  ctl_apply "$label"

  if [ -n "${HOTSPOT_SSID:-}" ]; then
    hw_step "checking the phone is on the hotspot"
    if hw_is_dry; then
      printf 'DRY  would assert the phone is joined to SSID %s\n' "$HOTSPOT_SSID"
    else
      # Join if a passphrase was exported for this run, then check. The check
      # is not skipped when the join reports success: ph_join drives a user
      # interface, and a tap that appeared to work is not evidence the phone
      # associated. Return 2 means no passphrase was given, which is the
      # join-by-hand path and not a failure.
      # Prove the hotspot exists before asking anything of the phone. A phone
      # that cannot find a network it was never offered is not a phone fault,
      # and without this the run blames the handset for a box with no radio in
      # access point mode.
      # The interface is DISCOVERED, not read from configuration. It used to
      # come from local/box.env, which said wlan0 because that was true when
      # the appliance took wlan0 over; once it began creating ap0 instead, the
      # run failed with "the Pi is not broadcasting Caspian-Wifi on wlan0"
      # while the hotspot was up and serving. The box decides which interface
      # hosts the hotspot, so the box is what gets asked.
      #
      # A configured HOTSPOT_IFACE is still honoured, because pinning it is how
      # somebody tests a specific arrangement on purpose. It is just no longer
      # the only source, and it is no longer consulted first.
      if pi_have; then
        hs_iface="$(pi_hotspot_iface || true)"
        if [ -z "$hs_iface" ]; then
          hw_die "no interface on the Pi is an access point, so there is no hotspot" \
            "nothing was asked of the phone. Read the advanced view or the service journal on the box: the hotspot did not come up, whatever the panel says."
        fi
        if [ -n "${HOTSPOT_IFACE:-}" ] && [ "$HOTSPOT_IFACE" != "$hs_iface" ]; then
          hw_info "the hotspot is on ${hs_iface}, not the configured ${HOTSPOT_IFACE}; using what the box is doing"
        fi
        if ! pi_hotspot_is_ap "$hs_iface" "$HOTSPOT_SSID"; then
          hw_die "the Pi is not broadcasting '$HOTSPOT_SSID' on $hs_iface" \
            "nothing was asked of the phone. Read the advanced view or the service journal on the box: the hotspot did not come up, whatever the panel says."
        fi
        HOTSPOT_IFACE="$hs_iface"
        export HOTSPOT_IFACE
      fi

      ph_join "$serial" "$HOTSPOT_SSID"
      case $? in
        0|2) : ;;
        *) hw_warn "the scripted join did not put the phone on '$HOTSPOT_SSID'" ;;
      esac
      ph_assert_ssid "$serial" "$HOTSPOT_SSID"
      hw_info "phone is on '$HOTSPOT_SSID'"
    fi
  else
    hw_warn "HOTSPOT_SSID is not set in local/box.env, so the harness cannot check the phone is on the hotspot."
    hw_warn "a capture taken while the phone is on its ordinary network will read as a LEAK and will be wrong."
  fi

  hw_step "capturing the exit IP through the tunnel"
  ph_cdp_up "$serial"
  line="$(ei_capture "$serial" "$dir")"
  ph_cdp_down "$serial"
  hw_info "capture: $line"

  if hw_is_dry; then hw_dry_stop "$run" "prove:$label"; return $?; fi

  a="$(ei_field a "$line")"; b="$(ei_field b "$line")"
  kind_a="$(ei_field a_kind "$line")"; kind_b="$(ei_field b_kind "$line")"
  void="$(ei_field void "$line")"

  if [ "$void" = "1" ]; then
    hw_ledger_end "$run" "prove:$label" "VOID"
    hw_say ""
    hw_say "RESULT VOID for '$label'."
    hw_say "  the phone changed network state during the capture, so the reading says nothing."
    hw_say "  retake it. This is NOT a leak and NOT a failure of the appliance."
    return "$HW_VOID"
  fi

  agree="$(ei_agree "$a" "$b")"
  ip="${agree#* }"
  [ "$ip" = '-' ] && ip=''

  # The third source, if the Pi is reachable. Independent of the phone and of
  # Chrome. It proves what the TUNNEL egresses as; it does not prove the phone
  # used it, and it is never allowed to stand in for the phone's own reading.
  socks=''
  if pi_have; then
    hw_info "cross-check: exit IP through the engine's own SOCKS listener on the Pi"
    socks="$(ei_value "$(ei_read_a "$(pi_socks_exit "$EI_ECHO_A_URL")")")"
    if [ -n "$socks" ]; then
      printf 'socks-source exit matched=%s\n' \
        "$( [ "$socks" = "$ip" ] && printf 'same as the phone' || printf 'DIFFERENT from the phone' )" \
        | hw_write "$dir/source-c-socks.txt"
      [ "$socks" = "$ip" ] || hw_warn "the Pi's own SOCKS probe egressed from a different address than the phone did. One of them is not going through the tunnel you think it is."
    else
      hw_info "the SOCKS cross-check produced nothing. Not fatal; it is a third source, not the result."
    fi
  fi

  classified="$(cfg_classify "$label" "$ip" "$baseline")"
  verdict="${classified%% *}"
  desc="${classified#* }"

  {
    printf 'label\t%s\n' "$label"
    printf 'scheme\t%s\n' "$(cfg_scheme "$label")"
    printf 'sources\t%s\n' "${agree%% *}"
    printf 'source_a_kind\t%s\n' "$kind_a"
    printf 'source_b_kind\t%s\n' "$kind_b"
    printf 'verdict\t%s\n' "$(hw_verdict_name "$verdict")"
    printf 'detail\t%s\n' "$desc"
  } | hw_write "$dir/result.tsv"

  if [ -n "$ip" ]; then
    fp="$(hw_fp "$ip" "$run")"
    printf '%s\n' "$fp" > "$dir/exit.fp"
    # The raw address goes to the scratch directory, never to the run directory:
    # a matching exit IP IS the config's server address, and the run directory is
    # the thing a person reads and copies out of. Only the fingerprint stays.
    printf '%s\n' "$ip" > "${HW_TMPDIR:?}/exit-$label.ip"
  fi

  hw_say ""
  case "$verdict" in
    "$HW_PASS")
      case "${agree%% *}" in
        agree)
          hw_say "RESULT PASS for '$label'."
          hw_say "  real traffic reached the internet through the tunnel and came back naming the box."
          hw_say "  box: $desc"
          hw_say "  two independent sources agreed (Chrome over HTTPS, and nc over HTTP)."
          ;;
        single-*)
          hw_say "RESULT SINGLE-SOURCE for '$label', which this project does not accept as a pass."
          hw_say "  the one source that answered matched the box '$desc'."
          hw_say "  the other source (A=$kind_a B=$kind_b) produced nothing, so the cached-page check did not happen."
          hw_ledger_end "$run" "prove:$label" "SINGLE-SOURCE"
          return "$HW_UNPROVEN"
          ;;
        disagree)
          hw_say "RESULT FAIL for '$label': the two sources reported different exit addresses."
          hw_say "  they are not averaged and one is not preferred. Something in the path is answering differently to different clients."
          hw_ledger_end "$run" "prove:$label" "FAIL"
          return "$HW_FAIL"
          ;;
      esac
      ;;
    "$HW_LEAK")
      hw_say "RESULT LEAK for '$label'. This outranks every other result in this run."
      hw_say "  the phone's exit IP through the appliance is the SAME address it has with no tunnel at all."
      hw_say "  client traffic is leaving by the uplink. Stop and fix this before measuring anything else."
      hw_say "  the phone's network state was stable across the whole capture, so this is not a VOID reading."
      ;;
    "$HW_UNPROVEN")
      hw_say "RESULT UNPROVEN for '$label'."
      hw_say "  no exit IP was captured (source A: $kind_a, source B: $kind_b)."
      hw_say "  this is NOT a pass. Nothing here says the transport works and nothing here says it does not."
      if [ "$kind_a" = "blocked" ]; then
        hw_say "  Chrome showed an error page, so the navigation genuinely failed rather than the readback failing."
      fi
      ;;
    "$HW_FAIL")
      hw_say "RESULT FAIL for '$label'."
      hw_say "  $desc"
      ;;
  esac

  hw_ledger_end "$run" "prove:$label" "$(hw_verdict_name "$verdict")"
  return "$verdict"
}
