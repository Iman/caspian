# shellcheck shell=bash
#
# Caspian-BYOC hardware harness: driving the box.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# This file deliberately contains no caspian command line for applying a config
# or starting and stopping the tunnel, because no such command line exists.
#
# That claim was checked twice on 2026-08-30, and it changed under this session,
# so the second reading is the one that counts:
#
#   FIRST READING, early in the session: cmd/caspian/ existed and was EMPTY.
#   `find cmd -type f` returned nothing.
#
#   SECOND READING, later the same session: another agent had populated it. The
#   usage text in cmd/caspian/main.go, function `usage`, now offers exactly
#   four things:
#     caspian serve --privileged
#     caspian serve --panel [--listen HOST:PORT]
#     caspian check                 read-only, changes nothing
#     caspian version
#   and ends with the sentence "After the installer has run, everything a
#   person does happens in the panel."
#
#   So there is still NO subcommand that applies a config, starts a tunnel or
#   stops one, and the CLI says in its own words that there is not meant to be.
#   The seam below is therefore not a workaround for missing code; it matches
#   what the product decided.
#
#   Because cmd/caspian is being written right now, this comment is dated and a
#   reader should re-run `caspian help` rather than trust it. It is cited here
#   only to record what the harness was built against.
#
#   The privileged side does have a closed action vocabulary, in
#   internal/panel/priv.go:58-81 -- detect, status, start, stop, engine-log --
#   but that is a Go interface reached over /run/caspian/priv.sock, and the wire
#   format on that socket is not something this session read. Guessing it would
#   produce a harness that fails against the real thing and blames the box.
#
#   The panel's own surface is HTML forms, not an API: internal/panel/panel.go
#   :128-146 lists POST /power, POST /config and the rest, every one of them
#   session-gated and, per panel.go:236-244, refused unless it passes the
#   same-origin check. GET /status.json is in that table WITHOUT Public:true, so
#   it needs a session too. Scripting that means holding a cookie and a
#   per-form token, against a package another agent is editing right now.
#
# So control is a seam. Two modes:
#
#   manual  the harness prints exactly what to do in the panel and waits. This
#           is the mode that works today, and it is honest about needing a
#           person.
#   hook    the harness sources local/control.local.sh, which the operator
#           writes once the command line exists. Four functions, named below.
#           This is the mode that makes an unattended run possible.
#
# --unattended with mode manual is refused rather than silently skipped.

CTL_MODE="${CASPIAN_HW_CONTROL:-manual}"
CTL_UNATTENDED="${CASPIAN_HW_UNATTENDED:-0}"

ctl_env_file() { printf '%s/box.env\n' "$(cfg_root)"; }
ctl_hook_file() { printf '%s/control.local.sh\n' "$(cfg_root)"; }

# ctl_load reads local/box.env for the few facts the harness cannot detect:
# the hotspot name to assert, the panel address to prove the hotspot is alive,
# and how to reach the Pi over ssh.
#
# HOTSPOT_SSID is not a secret. The WPA2 passphrase is, and this harness never
# needs it: the phone is joined by hand once and remembers the network.
ctl_load() {
  local f
  f="$(ctl_env_file)"
  if [ -f "$f" ]; then
    # shellcheck disable=SC1090
    . "$f"
  fi
  : "${HOTSPOT_SSID:=}"
  : "${PANEL_URL:=}"
  : "${PI_SSH:=}"
  : "${UPLINK_IFACE:=}"
  : "${HOTSPOT_IFACE:=ap0}"

  if [ "$CTL_MODE" = "hook" ]; then
    f="$(ctl_hook_file)"
    [ -f "$f" ] || hw_die "control mode is 'hook' but $f does not exist" \
      "write it, defining ctl_hook_apply, ctl_hook_start, ctl_hook_stop and ctl_hook_status. See docs/HARDWARE-TEST.md."
    # shellcheck disable=SC1090
    . "$f"
    for fn in ctl_hook_apply ctl_hook_start ctl_hook_stop ctl_hook_status; do
      command -v "$fn" >/dev/null 2>&1 \
        || hw_die "$f does not define $fn" "all four hook functions are required."
    done
  fi
}

_ctl_pause() {
  local prompt="$1"
  if [ "$CTL_UNATTENDED" = "1" ]; then
    hw_die "control mode 'manual' needs a person, and --unattended was given" \
      "either drop --unattended, or set CASPIAN_HW_CONTROL=hook and write local/control.local.sh."
  fi
  printf '\n     ACTION NEEDED: %s\n' "$prompt"
  printf '     press Enter when it is done, or Ctrl-C to abandon the run: '
  read -r _ignored || true
  printf '\n'
}

# ctl_apply puts one config into the box and brings the tunnel up with it.
ctl_apply() {
  local label="$1"
  hw_step "bringing the box up with config '$label'"
  if hw_is_dry; then
    printf 'DRY  would apply config %s and start the tunnel (mode: %s)\n' "$label" "$CTL_MODE" >&2
    return 0
  fi
  case "$CTL_MODE" in
    hook) ctl_hook_apply "$label" ;;
    manual)
      _ctl_pause "in the panel${PANEL_URL:+ at $PANEL_URL}: paste the config saved as local/configs/$label.conf, then turn the switch ON and wait for it to say connected"
      ;;
    *) hw_die "unknown control mode '$CTL_MODE'" "use manual or hook" ;;
  esac
}

ctl_stop() {
  hw_step "stopping the engine, leaving the hotspot up"
  if hw_is_dry; then
    printf 'DRY  would stop the engine and leave the access point running (mode: %s)\n' "$CTL_MODE" >&2
    return 0
  fi
  case "$CTL_MODE" in
    hook) ctl_hook_stop ;;
    manual)
      _ctl_pause "stop the ENGINE only. The hotspot must stay up and the phone must stay joined: the fail-closed test needs the phone still associated and still able to reach the panel. If the only control you have takes the hotspot down too, say so now and abandon this step rather than reporting it."
      ;;
  esac
}

ctl_start() {
  hw_step "starting the engine"
  if hw_is_dry; then
    printf 'DRY  would start the engine (mode: %s)\n' "$CTL_MODE" >&2
    return 0
  fi
  case "$CTL_MODE" in
    hook) ctl_hook_start ;;
    manual) _ctl_pause "turn the switch ON in the panel and wait for connected" ;;
  esac
}

# ctl_status prints whatever the control path can say. It is never used as
# proof: a status line is not an exit IP. It goes in the log so that a failure
# has context.
ctl_status() {
  if hw_is_dry; then
    printf 'DRY  would read the box status (mode: %s)\n' "$CTL_MODE" >&2
    return 0
  fi
  case "$CTL_MODE" in
    hook) ctl_hook_status 2>&1 || true ;;
    manual) printf '(manual control: no machine-readable status)\n' ;;
  esac
}
