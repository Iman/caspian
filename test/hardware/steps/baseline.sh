# shellcheck shell=bash
#
# Step 1: the baseline. A run with no baseline is not a run.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Taken with the phone on its ORDINARY network and NOT on the hotspot. It is the
# address the phone shows the internet when nothing is tunnelling it, and it is
# the single value that turns "the tunnel produced an exit IP" into "the tunnel
# produced a DIFFERENT exit IP". Without it, an appliance that forwards traffic
# straight out of its uplink produces a perfectly good exit IP and passes.
#
# It is stored two ways. The raw address goes to baseline.ip, because later
# steps must compare against it. A salted fingerprint goes to baseline.fp, and
# that is what any comparison writes down. The raw file is the ONE artefact
# written without redaction, and it is written before the address is registered
# as a secret, so every artefact after this point shows <baseline> instead.

step_baseline() {
  local serial="$1" run="$2" dir line a b kind_a kind_b void agree ip

  dir="$run/01-baseline"
  hw_ledger_start "$run" "baseline"

  hw_step "baseline: the phone's exit IP with NO tunnel"
  hw_info "the phone must be on its ordinary network now, not on the hotspot"

  if [ -n "${HOTSPOT_SSID:-}" ] && ! hw_is_dry; then
    local ssid
    ssid="$(ph_ssid "$serial")"
    if [ "$ssid" = "$HOTSPOT_SSID" ]; then
      hw_ledger_end "$run" "baseline" "PRECONDITION"
      hw_die "the phone is on the hotspot '$HOTSPOT_SSID' already" \
        "move it back to its ordinary network first. A baseline taken through the appliance is not a baseline, it is a second reading of the same path."
    fi
    hw_info "phone is on '$ssid', which is not the hotspot. Good."
  fi

  ph_cdp_up "$serial"
  line="$(ei_capture "$serial" "$dir")"
  ph_cdp_down "$serial"
  hw_info "capture: $line"

  if hw_is_dry; then hw_dry_stop "$run" "baseline"; return $?; fi

  a="$(ei_field a "$line")"; b="$(ei_field b "$line")"
  kind_a="$(ei_field a_kind "$line")"; kind_b="$(ei_field b_kind "$line")"
  void="$(ei_field void "$line")"

  if [ "$void" = "1" ]; then
    hw_ledger_end "$run" "baseline" "VOID"
    hw_err "the phone changed network state while the baseline was being taken"
    hw_err "VOID. Retake it. This is not a failure of the appliance."
    return "$HW_VOID"
  fi

  agree="$(ei_agree "$a" "$b")"
  ip="${agree#* }"
  case "$agree" in
    agree*)
      hw_info "both sources agree"
      ;;
    disagree*)
      hw_ledger_end "$run" "baseline" "FAIL"
      hw_err "the two sources reported DIFFERENT addresses for an untunnelled phone"
      hw_err "  source A (Chrome, HTTPS): $a   [$kind_a]"
      hw_err "  source B (nc, HTTP):      $b   [$kind_b]"
      hw_err "that is a finding about the network the phone is on, not about the appliance."
      hw_err "settle it before going further: a baseline nobody can reproduce grades every later step wrong."
      return "$HW_FAIL"
      ;;
    single-*)
      hw_warn "only one source answered (A=$kind_a B=$kind_b). The baseline is SINGLE-SOURCE."
      hw_warn "it can still be used, and every result compared against it inherits that weakness."
      ;;
    none*)
      hw_ledger_end "$run" "baseline" "UNPROVEN"
      hw_err "no exit IP was captured at all (A=$kind_a B=$kind_b)"
      hw_err "UNPROVEN. Check the phone actually has internet on its ordinary network:"
      hw_err "  adb -s $serial shell cmd wifi status"
      return "$HW_UNPROVEN"
      ;;
  esac

  hw_require_measured "$ip"

  # Raw, deliberately unredacted, written before registration. See the header.
  printf '%s\n' "$ip" > "$run/baseline.ip"
  hw_fp "$ip" "$run" > "$run/baseline.fp"
  hw_secret "$ip" "<baseline>"

  hw_say ""
  hw_say "BASELINE captured: $ip"
  hw_say "  stored raw in    $run/baseline.ip  (inside the gitignored /local/ tree)"
  hw_say "  fingerprint      $(cat "$run/baseline.fp")"
  hw_say "  from here on it appears in every artefact as <baseline>"
  hw_ledger_end "$run" "baseline" "OK"
  return "$HW_PASS"
}

# hw_fp is a salted fingerprint of an address.
#
# It exists so that "the exit IP changed" can be written down without writing an
# address down. The salt is per-run and lives only in the run directory, so the
# fingerprints in a summary cannot be walked back to an address by trying all
# four billion of them.
hw_fp() {
  local value="$1" run="$2" salt
  if [ ! -f "$run/.salt" ]; then
    if command -v openssl >/dev/null 2>&1; then
      openssl rand -hex 16 > "$run/.salt" 2>/dev/null
    else
      # /dev/urandom is present on both macOS and Raspberry Pi OS.
      od -An -N16 -tx1 /dev/urandom | tr -d ' \n' > "$run/.salt"
    fi
    chmod 600 "$run/.salt" 2>/dev/null || true
  fi
  salt="$(cat "$run/.salt")"
  if command -v shasum >/dev/null 2>&1; then
    printf '%s%s' "$salt" "$value" | shasum -a 256 | cut -c1-16
  else
    printf '%s%s' "$salt" "$value" | sha256sum | cut -c1-16
  fi
}

# step_baseline_load reads the baseline back for a later step, and refuses when
# there is none.
step_baseline_load() {
  local run="$1"
  if hw_is_dry; then printf '%s\n' "$HW_DRY_SENTINEL"; return 0; fi
  [ -f "$run/baseline.ip" ] || hw_die "this run has no baseline" \
    "run 'caspian-hw baseline' first, with the phone on its ordinary network. Every later verdict is a comparison against it, so there is nothing to compare without one."
  cat "$run/baseline.ip"
}
