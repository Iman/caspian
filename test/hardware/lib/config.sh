# shellcheck shell=bash
#
# Caspian-BYOC hardware harness: the configs, and what an exit IP is allowed to
# mean.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Reads real share links out of /local/, which is gitignored. Nothing in this
# file ever writes a config, a host, a user id or a key anywhere. It registers
# all of them as secrets with lib/common.sh first, so that if a later step is
# careless the redaction filter catches it and hw_guard deletes the file.
#
# Layout under the repository root:
#
#   local/configs/<label>.conf   one share link, one file, one line
#   local/boxes.tsv              <label> <TAB> <addr>[,<addr>...]   (optional)
#
# The label is the filename with .conf removed. It is the ONLY name that ever
# reaches a log, a summary or a directory name, which is why hw_guard_name
# refuses a label that has an address in it.
#
# boxes.tsv exists for the case the parser cannot cover: a server whose EGRESS
# address differs from the address in the link. That is normal (multi-homed
# boxes, CDN ingress, NAT), and without it the harness would call a working
# tunnel a failure. It is operator-maintained, it is authority for the name,
# and the parsed host is cross-checked against it so a stale row is caught.

# ---------------------------------------------------------------------------
# Scheme gate.
#
# Mirrored from internal/link/link.go, `supportedSchemes` at :116-124, which is
# itself derived from third_party/libxray-share/parse_share.go. It is repeated
# here rather than imported because this is shell, and cfg_selftest_schemes
# re-reads the Go source and fails if the two lists have drifted. A comment
# claiming they agree would not survive the first edit; a check does.
#
# Out of scope, and named so the harness can say WHICH thing it will not do:
# the vendored parser does not handle tuic, ssr, wireguard or anytls
# (docs/2026-08-29-design.md section 4.4).
# ---------------------------------------------------------------------------
CFG_SCHEMES_IN='vless vmess ss socks trojan hysteria2 hy2'
CFG_SCHEMES_OUT='tuic ssr ss2022 wireguard wg anytls naive brook juicity'

# CASPIAN_HW_LOCAL exists so the offline selftest can point the whole config
# layer at a scratch tree of synthetic links. It is not for real use: the
# default is the gitignored /local/ and there is no reason to move it.
cfg_root() { printf '%s\n' "${CASPIAN_HW_LOCAL:-$(hw_repo_root)/local}"; }
cfg_dir()  { printf '%s/configs\n' "$(cfg_root)"; }
cfg_boxes(){ printf '%s/boxes.tsv\n' "$(cfg_root)"; }

cfg_path() { printf '%s/%s.conf\n' "$(cfg_dir)" "$1"; }

# cfg_list prints every label found, one per line.
cfg_list() {
  local d
  d="$(cfg_dir)"
  [ -d "$d" ] || return 0
  find "$d" -maxdepth 1 -type f -name '*.conf' 2>/dev/null \
    | sed -e 's|^.*/||' -e 's/\.conf$//' | sort
}

cfg_require() {
  local label="$1" p
  hw_guard_name "$label"
  p="$(cfg_path "$label")"
  [ -f "$p" ] || hw_die "no config named '$label'" \
    "expected $p. Put one share link in it. Run 'caspian-hw configs' to list what is there."
  [ -s "$p" ] || hw_die "the config '$label' is empty" "put one share link in $p"
}

# cfg_raw prints the single share link. Never call this into a file.
cfg_raw() {
  sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' "$(cfg_path "$1")" \
    | grep -v '^#' | grep -v '^$' | head -1
}

cfg_scheme() {
  cfg_raw "$1" | sed -n 's|^\([A-Za-z][A-Za-z0-9+.-]*\)://.*|\1|p' | tr '[:upper:]' '[:lower:]'
}

# cfg_assert_supported refuses a transport this engine does not carry, and says
# which one it was, before anything is started. The scheme is a vocabulary word
# and not credential material, so naming it discloses nothing; internal/link
# makes the same call at link.go:176-180.
cfg_assert_supported() {
  local label="$1" scheme
  scheme="$(cfg_scheme "$label")"
  if [ -z "$scheme" ]; then
    hw_die "the config '$label' does not begin with a scheme like vless://" \
      "this harness takes one share link per file. Raw xray JSON, a base64 subscription blob and Clash YAML are all things internal/link accepts and this harness does not: it needs the server address to compare an exit IP against."
  fi
  case " $CFG_SCHEMES_IN " in
    *" $scheme "*) printf '%s\n' "$scheme"; return 0 ;;
  esac
  case " $CFG_SCHEMES_OUT " in
    *" $scheme "*)
      hw_die "'$label' is a $scheme:// link, which xray-core does not carry" \
        "out of scope for this appliance. In scope: $CFG_SCHEMES_IN. See docs/2026-08-29-design.md section 4.4."
      ;;
  esac
  hw_die "'$label' uses the scheme '$scheme', which this harness does not know" \
    "in scope: $CFG_SCHEMES_IN. If xray-core really does carry it, add it to CFG_SCHEMES_IN and to internal/link supportedSchemes together."
}

# ---------------------------------------------------------------------------
# Host extraction.
#
# Narrow on purpose. Its only job is to produce a candidate server address so
# that boxes.tsv can be cross-checked and so that a config with no boxes.tsv row
# can still be graded. When it cannot tell, it says nothing and the caller falls
# back to boxes.tsv, which is the authority anyway.
# ---------------------------------------------------------------------------

cfg_b64d() {
  local data pad
  data="$(tr -d '\n\r' | tr -- '-_' '+/')"
  pad=$(( ${#data} % 4 ))
  case "$pad" in
    2) data="${data}==" ;;
    3) data="${data}=" ;;
    1) return 1 ;;
  esac
  printf '%s' "$data" | base64 -d 2>/dev/null || printf '%s' "$data" | base64 -D 2>/dev/null
}

# _cfg_authority_host takes scheme://[userinfo@]host[:port][?..][#..] and prints
# host. Bracketed IPv6 is unwrapped.
_cfg_authority_host() {
  local rest
  rest="$(printf '%s' "$1" | sed -e 's|^[A-Za-z][A-Za-z0-9+.-]*://||' -e 's|[?#].*$||')"
  rest="${rest##*@}"
  case "$rest" in
    '['*) printf '%s\n' "$rest" | sed -e 's|^\[||' -e 's|\].*$||' ;;
    *)    printf '%s\n' "${rest%%:*}" ;;
  esac
}

# cfg_host prints the server host from the link, or nothing.
cfg_host() {
  local label="$1" raw scheme decoded
  raw="$(cfg_raw "$label")"
  scheme="$(cfg_scheme "$label")"
  case "$scheme" in
    vmess)
      # vmess:// is base64 of a JSON object; the address is "add".
      decoded="$(printf '%s' "$raw" | sed 's|^[Vv][Mm][Ee][Ss][Ss]://||' | cfg_b64d)" || return 0
      printf '%s' "$decoded" \
        | tr ',' '\n' \
        | sed -n 's/.*"add"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
        | head -1
      ;;
    ss)
      # Two shapes. ss://b64(method:pass)@host:port and, older,
      # ss://b64(method:pass@host:port). The second has no @ once the scheme is
      # gone, so decode and re-read.
      if printf '%s' "$raw" | sed 's|^[Ss][Ss]://||' | sed 's|[?#].*$||' | grep -q '@'; then
        _cfg_authority_host "$raw"
      else
        decoded="$(printf '%s' "$raw" | sed -e 's|^[Ss][Ss]://||' -e 's|[?#].*$||' | cfg_b64d)" || return 0
        _cfg_authority_host "ss://$decoded"
      fi
      ;;
    vless|trojan|socks|hysteria2|hy2)
      _cfg_authority_host "$raw"
      ;;
  esac
}

# ---------------------------------------------------------------------------
# Secret registration.
#
# Everything derived from a config becomes a redaction rule before any step
# runs. Deliberately over-broad: the whole raw link, the whole userinfo, the
# host, and every query-parameter value of four characters or more. A REALITY
# public key, a short id and a UUID all arrive as query values, so covering the
# whole class is more reliable than naming each one and missing the next.
# ---------------------------------------------------------------------------
#
# Query-parameter values get a HIGHER minimum length than hosts and
# credentials. That is not laziness, it is a defect fix: a REALITY link carries
# security=reality and fp=chrome, and registering those as secrets redacted the
# word "reality" out of every artefact, including the label of the config whose
# name the report is required to print. Eight characters keeps the key material
# (pbk, sid, spx, pqv, sni, host, path are all longer) and drops the vocabulary.
# hw_protect is the belt to this braces: a value that appears inside a label is
# refused whatever its length.
CFG_PARAM_MIN_LEN=8

cfg_register_secrets() {
  local label="$1" raw host userinfo v
  raw="$(cfg_raw "$label")"
  [ -n "$raw" ] || return 0

  hw_secret "$raw" "<config:$label>"

  userinfo="$(printf '%s' "$raw" | sed -n 's|^[A-Za-z][A-Za-z0-9+.-]*://\([^@/?#]*\)@.*|\1|p')"
  [ -n "$userinfo" ] && hw_secret "$userinfo" "<credential:$label>"

  host="$(cfg_host "$label")"
  [ -n "$host" ] && hw_secret "$host" "<box:$label>"

  # printf '%s\n', with the newline, and the "|| [ -n "$kv" ]" on the read.
  # Both are the same defect fixed twice: without a trailing newline the last
  # parameter of every link was silently dropped and never registered, so the
  # last query value in a share link went unredacted. Caught by the selftest on
  # 2026-08-30.
  printf '%s\n' "$raw" \
    | sed -n 's|^[^?]*?\([^#]*\).*|\1|p' \
    | tr '&' '\n' \
    | while IFS= read -r kv || [ -n "$kv" ]; do
        v="${kv#*=}"
        [ -n "$v" ] && hw_secret "$v" "<param:$label>" "$CFG_PARAM_MIN_LEN"
      done

  # Addresses from boxes.tsv are server addresses too. Registering them means a
  # matching exit IP is rendered as <box:label> in every artefact, which is
  # exactly the required behaviour: name the box, never the address.
  cfg_addresses "$label" | while IFS= read -r a || [ -n "$a" ]; do
    [ -n "$a" ] && hw_secret "$a" "<box:$label>"
  done
}

# cfg_register_all_secrets covers every config present, not just the one under
# test. Without it, an exit IP that lands on a DIFFERENT box would be printed
# raw, which discloses another config's server address.
#
# Every label is protected FIRST. Labels are the only names the harness is
# allowed to print, so nothing may be registered that would eat one.
cfg_register_all_secrets() {
  local l
  for l in $(cfg_list); do
    [ -n "$l" ] && hw_protect "$l"
  done
  for l in $(cfg_list); do
    [ -n "$l" ] && cfg_register_secrets "$l"
  done
}

# ---------------------------------------------------------------------------
# Addresses and names.
# ---------------------------------------------------------------------------

# cfg_boxes_addresses prints the addresses boxes.tsv gives a label.
cfg_boxes_addresses() {
  local label="$1" f
  f="$(cfg_boxes)"
  [ -f "$f" ] || return 0
  awk -F'\t' -v l="$label" '
    /^[[:space:]]*#/ { next }
    $1 == l { n = split($2, a, ","); for (i = 1; i <= n; i++) { gsub(/[[:space:]]/, "", a[i]); if (a[i] != "") print a[i] } }
  ' "$f"
}

# cfg_resolve prints the A and AAAA records for a name, and says nothing when it
# cannot resolve. The vantage is the machine running the harness, which is NOT
# the vantage the tunnel dials from; a geo-routed name can answer differently on
# the Pi. That is what boxes.tsv is for, and cfg_addresses prefers it.
cfg_resolve() {
  local host="$1"
  [ -n "$host" ] || return 0
  case "$host" in
    *[!0-9.]*) : ;;
    *) printf '%s\n' "$host"; return 0 ;;   # already an IPv4 literal
  esac
  case "$host" in
    *:*) printf '%s\n' "$host"; return 0 ;; # already an IPv6 literal
  esac
  if command -v python3 >/dev/null 2>&1; then
    python3 - "$host" <<'PY' 2>/dev/null
import socket, sys
seen = []
try:
    for family, _, _, _, addr in socket.getaddrinfo(sys.argv[1], None):
        ip = addr[0]
        if ip not in seen:
            seen.append(ip)
except OSError:
    pass
for ip in seen:
    print(ip)
PY
    return 0
  fi
  command -v dig >/dev/null 2>&1 && dig +short A "$host" AAAA "$host" 2>/dev/null | grep -E '^[0-9a-fA-F.:]+$'
}

# cfg_addresses prints every address that counts as "this box", newline
# separated. boxes.tsv first, then whatever the link's host resolves to.
cfg_addresses() {
  local label="$1" host
  cfg_boxes_addresses "$label"
  host="$(cfg_host "$label")"
  [ -n "$host" ] && cfg_resolve "$host"
}

# cfg_crosscheck warns when boxes.tsv and the link disagree about the host. It
# warns rather than failing, because a differing egress address is the normal
# reason for a boxes.tsv row to exist. Silence here would let a stale row
# quietly rename a box.
cfg_crosscheck() {
  local label="$1" host listed
  host="$(cfg_host "$label")"
  listed="$(cfg_boxes_addresses "$label")"
  [ -n "$listed" ] || return 0
  [ -n "$host" ] || return 0
  if ! cfg_resolve "$host" | grep -qxF -f <(printf '%s\n' "$listed") 2>/dev/null; then
    hw_warn "boxes.tsv for '$label' lists no address that the link's host resolves to from this machine."
    hw_warn "that is expected when the box egresses from a different address. It is a STALE ROW when it is not."
  fi
}

# cfg_label_for_ip prints the label of whichever config owns an address, or
# nothing. This is what stops the harness printing one config's server address
# while reporting another config's failure.
cfg_label_for_ip() {
  local ip="$1" l
  [ -n "$ip" ] || return 0
  cfg_list | while IFS= read -r l; do
    [ -n "$l" ] || continue
    if cfg_addresses "$l" | grep -qxF "$ip"; then
      printf '%s\n' "$l"
      return 0
    fi
  done
}

# ---------------------------------------------------------------------------
# The verdict.
#
# cfg_classify <label> <exit-ip> <baseline-ip>
#
# Prints "<code> <description>" and returns nothing else. The description is
# always safe to print: it is a label, or an address that belongs to no config.
#
# Order is the whole point. A leak is checked FIRST, because an exit IP equal to
# the untunnelled baseline outranks every other reading, including a reading
# that would otherwise have been a pass.
# ---------------------------------------------------------------------------
cfg_classify() {
  local label="$1" ip="$2" baseline="$3" owner

  if [ -z "$ip" ]; then
    printf '%s %s\n' "$HW_UNPROVEN" "no exit IP was captured"
    return 0
  fi
  if [ -n "$baseline" ] && [ "$ip" = "$baseline" ]; then
    printf '%s %s\n' "$HW_LEAK" "the exit IP equals the untunnelled baseline"
    return 0
  fi
  if cfg_addresses "$label" | grep -qxF "$ip"; then
    printf '%s %s\n' "$HW_PASS" "$label"
    return 0
  fi
  owner="$(cfg_label_for_ip "$ip")"
  if [ -n "$owner" ]; then
    printf '%s %s\n' "$HW_FAIL" "the exit IP belongs to '$owner', not to '$label'"
    return 0
  fi
  printf '%s %s\n' "$HW_FAIL" "the exit IP $ip matches no configured box"
}

# ---------------------------------------------------------------------------
# Drift check. Run by selftest, offline, no device needed.
# ---------------------------------------------------------------------------
cfg_selftest_schemes() {
  local go_file go_list mine
  go_file="$(hw_repo_root)/internal/link/link.go"
  [ -f "$go_file" ] || { printf 'SKIP internal/link/link.go not present\n'; return 0; }
  go_list="$(sed -n '/^var supportedSchemes = map\[string\]bool{/,/^}/p' "$go_file" \
             | sed -n 's/^[[:space:]]*"\([a-z0-9]*\)":.*/\1/p' | sort | tr '\n' ' ')"
  mine="$(printf '%s' "$CFG_SCHEMES_IN" | tr ' ' '\n' | grep -v '^$' | sort | tr '\n' ' ')"
  if [ "$go_list" = "$mine" ]; then
    printf 'ok   scheme list matches internal/link supportedSchemes: %s\n' "$mine"
    return 0
  fi
  printf 'FAIL scheme drift\n  go:      %s\n  harness: %s\n' "$go_list" "$mine"
  return 1
}
