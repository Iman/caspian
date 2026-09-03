# Installing Caspian-BYOC

[🇮🇷 فارسی](INSTALL.fa.md) | 🇬🇧 **English** | [🇷🇺 Русский](../README.ru.md) | [🇨🇳 中文](../README.zh.md)

> Persian edition: [`docs/INSTALL.fa.md`](INSTALL.fa.md). The English file is
> the one the tests read. If the two ever disagree, this one is correct.

One command, and then the panel.

    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"

The owner is `Iman`, in `docs/LAYOUT.md` and in `install.sh`, where
`CASPIAN_ORG` defaults to it. The artefacts and the `SHA256SUMS` file are built
and published by `.github/workflows/release.yml` when a version tag is pushed.

After the installer finishes it prints two things, the panel address and a
first-run password, and nothing else that matters. Every further action happens
in the panel.

## What it does, in order

1. Refuses anything it cannot install onto, before touching the machine.
   Not Linux, an architecture with no artefact, no systemd, or systemd older
   than 240. Each refusal names what was found and what is supported, and exits
   non-zero.
2. Works out whether this is a fresh install or an upgrade, by looking for
   `/usr/local/bin/caspian`.
3. Finds the package manager and installs only the missing dependencies. It
   lists them first, and asks before installing when there is a terminal to ask
   at.
4. Downloads the release artefact and the checksums file, verifies SHA-256, and
   refuses on any mismatch. This happens before anything on the box is stopped
   or replaced, so a failed download leaves a working installation exactly as it
   was.
5. Stops `caspian-panel.service` and then `caspian.service`, if they are there.
6. Creates the `caspian` system group and user.
7. Creates `/var/lib/caspian`, `/run/caspian` and `/run/caspian/dnsmasq` with
   the modes in `docs/LAYOUT.md`. It never touches the contents of
   `/var/lib/caspian`.
8. Installs the binary to `/usr/local/bin/caspian`.
9. Writes the two systemd units, the tmpfiles fragment and the modules-load
   fragment, then reloads systemd, creates `/run/caspian` and loads the `tun`
   and `nf_tables` modules.
10. On a fresh install only, generates a first-run password and leaves it where
    the panel will pick it up.
11. Enables and starts both units.
12. Copies the uninstaller to `/usr/local/bin/caspian-uninstall`, if it can.
13. Prints the panel address and the password.

## Requirements, and what it refuses

| Requirement | Refusal message names |
|---|---|
| Linux | what `uname -s` said |
| x86_64, aarch64, armv7l or armv6l | what `uname -m` said |
| systemd, version 240 or newer | the contents of `/proc/1/comm`, or the version found |
| root | what to run instead |

systemd 240 is the version that introduced `Type=exec`, which both units use.

`armv8l` is deliberately not mapped. It is a 32-bit userland on a 64-bit
kernel, `docs/LAYOUT.md` does not say which artefact it should take, and
guessing is how the armv6 bug below happened. It refuses and says so.

## Architecture mapping, and the release-side requirement that goes with it

The release artefacts follow Go's names, not the kernel's:

| `uname -m` | Artefact |
|---|---|
| `x86_64` | `caspian-linux-amd64` |
| `aarch64` | `caspian-linux-arm64` |
| `armv7l` | `caspian-linux-arm` |
| `armv6l` | `caspian-linux-arm` |

A previous project in this workspace mapped armv6 onto an armv7 artefact and
broke the older Pi models. An armv7 build uses instructions the ARM1176 in a
Pi 1, a Pi Zero or a Pi Zero W does not have, so the binary installs cleanly and
then dies with an illegal instruction the first time it runs.

Both 32-bit values map onto one artefact here, which is what `docs/LAYOUT.md`
fixes. That is only correct while `caspian-linux-arm` is built with `GOARM=6`.

**Building it `GOARM=7` puts the same bug back one layer up, in the release
pipeline instead of in the installer, where no test in this repository can see
it.** The release build must set `GOARM=6` for the `linux/arm` artefact.

## Dependencies

From `docs/LAYOUT.md`: `hostapd`, `dnsmasq`, `nftables`, `iw`, `iproute2`.

The installer tests for the command each package provides rather than asking a
package database, because "is `nft` on this box" has the same answer everywhere:

| Package | Command tested |
|---|---|
| hostapd | `hostapd` |
| dnsmasq | `dnsmasq` |
| nftables | `nft` |
| iw | `iw` |
| iproute2 | `ip` |

Package managers are detected in this order: `apt-get`, `dnf`, `yum`, `pacman`,
`zypper`, `apk`. One package name differs between them: on `dnf` and `yum`,
`iproute2` is called `iproute`.

If a package name turns out to be wrong on some distribution, the failure is a
message naming the command that is still missing and the package that was
tried, followed by a refusal to continue. It is never a silent half-install.

## The download, and what the checksum does and does not prove

The artefact and a `SHA256SUMS` file are fetched from the same release
directory. The file is in `sha256sum` format, one line per artefact:

    <64 hex characters>  caspian-linux-arm64

The installer refuses four things, with four different messages, because they
are four different problems: no entry for this artefact, an entry that is not a
SHA-256 hash, a hash that does not match, and a URL that is not HTTPS.

What the check proves: the artefact is the one the checksums file describes. It
catches a truncated download, a corrupted mirror and a stale CDN copy.

What it does not prove: both files come from the same place, so somebody who
controls that place can serve a matching pair. HTTPS to the release host is what
defends against that, which is why a plaintext base URL is refused unless
`CASPIAN_ALLOW_INSECURE_URL=1` is set for local testing.

## Running it twice

A second run is an upgrade. It stops the services, replaces the binary,
rewrites the units, and restarts. It does not touch `/var/lib/caspian`, so the
proxy config, the hotspot name and password, and the panel password all survive.
It generates no new password and says `Password: unchanged from the previous
install`.

## What gets created

| Path | Mode | Owner | What |
|---|---|---|---|
| `/usr/local/bin/caspian` | 0755 | root | The single binary |
| `/usr/local/bin/caspian-uninstall` | 0755 | root | A local copy of `uninstall.sh` |
| `/var/lib/caspian` | 0700 | caspian | State. Never removed on upgrade |
| `/run/caspian` | 0750 | root:caspian | Runtime, recreated at every boot |
| `/run/caspian/dnsmasq` | 0700 | caspian | dnsmasq's pid file, recreated at every boot |
| `/etc/systemd/system/caspian.service` | 0644 | root | Privileged unit |
| `/etc/systemd/system/caspian-panel.service` | 0644 | root | Panel unit |
| `/etc/tmpfiles.d/caspian.conf` | 0644 | root | Recreates both `/run` directories at boot |
| `/etc/modules-load.d/caspian.conf` | 0644 | root | Loads `tun` and `nf_tables` at boot |

There is no `/etc/caspian`. It was in the layout until 2026-08-30 and was
removed, because the generated hostapd and dnsmasq files it was supposed to hold
live under `/run`: they are rewritten on every start, they carry the WPA2
passphrase, and `/run` is a tmpfs, so a credential does not persist into a file
nobody knows is there.

`/run` being a tmpfs is also why neither runtime directory can be created once
by an installer and expected to survive a reboot. The tmpfiles fragment recreates
both, with exactly the modes and ownership the layout fixes.

Three more paths appear at runtime and are not the installer's to make:
`/run/caspian/hostapd.conf` and `/run/caspian/dnsmasq.conf` (0600 root, rewritten
on every start) and `/run/caspian/dnsmasq/dnsmasq.pid` (0644 caspian, written by
dnsmasq itself).

### Why dnsmasq gets its own directory, and the trap beside it

dnsmasq drops to the `caspian` account and then writes its pid file.
`/run/caspian` is 0750 root:caspian, so the group can list it and cannot write in
it, and whether dnsmasq writes the pid before or after it drops privileges is a
property of dnsmasq that nobody here has measured. Giving dnsmasq a directory it
owns means the answer stops mattering, which is better than measuring it once and
then depending on it staying true across a dnsmasq upgrade.

**Do not fix a pid file that will not write by making `/run/caspian`
group-writable.** Permission to create and delete inside a directory comes from
the directory, not from the file, so a group-writable `/run/caspian` would let the
unprivileged panel account delete `hostapd.conf` and write its own, which the
privileged side then hands to hostapd running as root. That turns a pid-file
inconvenience into local privilege escalation. The same warning is in
`docs/LAYOUT.md`, in `install.sh` beside the line that creates the directory, and
in `packaging/caspian.tmpfiles.conf` beside the line that recreates it, because
that is where somebody will be standing when they hit it.

## Ports

The installer sets no port and checks none. It prints one, the panel's, and takes
it from the table in `docs/LAYOUT.md`, "Ports", rather than from memory:

| Port | Bind | What | Agrees with |
|---|---|---|---|
| 53 | hotspot interface | dnsmasq, DHCP and DNS for joined devices | `internal/netcfg/plan.go`, `DNSPort` |
| 5354 | 127.0.0.1 | The engine's local DNS listener | `internal/xcfg`, `DefaultLocalDNSPort` |
| 8088 | panel address | The web panel | `internal/netcfg/plan.go`, `PanelPort` |
| 10808 | 127.0.0.1 | SOCKS, for diagnostics and the exit-IP proof | `internal/xcfg`, `DefaultSocksPort` |

The one that breaks quietly is 5354: dnsmasq forwards only there and the engine
listens there, and if the two drift, DNS stops resolving for every joined device
while the hotspot and the tunnel both still look healthy. Neither end of that
pairing is set by the installer, and the cross-check for it belongs in
`internal/xcfg`, where the test already lives.

## The two units

`caspian.service` runs as root and owns routes, the firewall, the access point
and the engine. `caspian-panel.service` runs as `caspian` and owns the web
interface and nothing privileged. The panel is ordered after the privileged
service with `Wants=`, never `Requires=`. Design section 5.6 records that a user
who cannot reach the panel cannot fix anything, so the panel has to come up and
say what is wrong even when the privileged side failed to start.

The unit files in `packaging/` are the source of truth. `install.sh` carries a
byte-identical copy of each one inline, because a script piped from `curl` has
no repository to read from, and because downloading them would add unverified
artefacts beside the one the installer goes to the trouble of checksumming.
`packaging/test-install.sh` fails if a copy drifts.

Both units are hardened, and each directive in them says what it protects
against. Four directives are switched **off** in the privileged unit, on
purpose, and each says what it would have broken:

| Not set | What it would have broken |
|---|---|
| `PrivateDevices=yes` | Removes `/dev/net/tun`, so the engine cannot create the tunnel. Replaced with `DevicePolicy=closed` plus `DeviceAllow` for `/dev/net/tun` and `/dev/rfkill` |
| `ProtectKernelTunables=yes` | Mounts `/proc/sys` read-only, so `rp_filter` cannot be set. Design section 4.2: a wrong `rp_filter` gives a tunnel that connects and carries nothing |
| `ProtectKernelModules=yes` | Denies the on-demand module loading the nftables ruleset triggers |
| `ProtectProc=invisible` | Hides other processes from `pgrep`, so the supervisor can never find a stray `hostapd` or `dnsmasq` holding the radio |

The panel unit has none of those constraints, so it is as tight as systemd
allows: no capabilities in either set, `PrivateDevices=yes`, `ProtectProc=invisible`,
`ProcSubset=pid`, `MemoryDenyWriteExecute=yes`, and no `AF_NETLINK`.

## First run

The installer prints:

    Panel:    http://192.168.4.31:8088/
    Password: nvbqd-3kx7m-rjhta-92wpe

That address and that password are the whole handover. Everything after them
happens on this page:

![The panel, with the tunnel up, before any device has joined](images/panel-en.png)

The port is 8088, from `docs/LAYOUT.md`, "Ports". See the Ports section above.

The address is the box's current global IPv4 address. It is a best effort, and
the reason is worth knowing: design section 5.6 says the panel listens on the
hotspot interface, and the hotspot does not exist until the user switches it on,
so at the end of an install the only address that exists is the one the box
already had. Section 5.6 records this as a hazard that v1 still has to answer.

The password is twenty characters from a thirty-two character alphabet, which is
a hundred bits. The alphabet leaves out `0`, `O`, `1`, `l` and `I`, because
somebody who did not choose this password has to read it off a terminal and type
it into a phone.

### The handoff, which needs the panel to cooperate

The installer writes the plaintext to `/var/lib/caspian/first-run-password`,
mode 0600, owned by `caspian`, inside a 0700 directory. The contract is:

> On its first start, the panel reads that file, passes the contents to
> `state.Store.SetPanelPassword`, which hashes it with argon2id, and then
> deletes the file.

A file rather than a command-line argument or an environment variable, because
both of those are readable from `/proc` by anything on the box.

**This half is implemented.** `cmd/caspian/firstrun.go` provides
`consumeFirstRunPassword`, `cmd/caspian/serve_panel.go` calls it at panel start,
and `cmd/caspian/firstrun_test.go` covers it. The printed password works.

That paragraph used to say the opposite: that `cmd/` was empty, nothing consumed
the file, and the printed password did not work. It was written before `019fba6`
populated `cmd/caspian` on 2026-08-30 and nobody came back to it. Corrected
2026-08-31. Two other documents recorded the change on the day and this one did
not, which is the ordinary way a document goes stale: the person making the
change updates the file they are looking at. Two things already in the tree meet it halfway:
`state.Store.SetPanelPassword` exists, and `state.ErrNoPanelPassword` is
documented as the signal for the panel to show a setup screen, which is the
right fallback for when the file is absent.

The installer never reads, prints or logs the user's proxy config. It has no
reason to: the only thing that holds a config is the panel.

## Uninstalling

    sudo /usr/local/bin/caspian-uninstall

or, the same way the installer is fetched:

    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/uninstall.sh)"

The installer keeps a local copy on the box on purpose. Somebody who wants to
uninstall very often wants it because the box's networking is in a state they do
not like, and that is the worst moment to need a working network to fetch a
script.

It stops and disables both units, replays the network journal, removes the units,
the binary and the runtime directories, and then asks about state. One `rm -rf` of
`/run/caspian` takes the dnsmasq directory inside it as well.

| Flag | Effect |
|---|---|
| `--dry-run` | Print every action, take none |
| `--purge` | Delete `/var/lib/caspian` without asking. Also removes the account |
| `--keep-state` | Keep `/var/lib/caspian` without asking |
| `--force` | Carry on even when the network journal cannot be replayed |
| `--show-commands` | Print each replayed command in full. See the warning below |
| `--yes`, `-y` | Do not ask anything |

With neither `--purge` nor `--keep-state`, and no terminal to ask at, the state
is kept. A deleted config cannot be got back and a kept one costs a directory.

The account is removed only when the state directory goes with it. Removing the
account while its files are still there would leave them owned by a numeric id
that the next account created on the box could inherit.

### Replaying the journal

The journal is `/var/lib/caspian/netcfg.journal`. `docs/LAYOUT.md` and
`internal/netcfg/journal.go` (`DefaultJournalPath`) agree on that name; they did
not until 2026-08-30, when the layout still called it `teardown.journal`. An
uninstaller that looks for the wrong name silently leaves the routes, rules and
firewall in place while telling the user the network was restored, so a test
asserts the name the script looks for.

It holds the inverse of every network change the privileged service made. Each
inverse is written to disk before the change reaches the kernel, so a process
that was killed rather than stopped still leaves a way back (design section 5.5,
and the `Applier` contract in `internal/netcfg/apply.go`).

The on-disk format is JSON lines, one `Record` per line, several lines per step,
keyed by `seq`:

    {"seq":2,"phase":"begin","t":"...","op":"route","why":"...",
     "do":{"path":"ip","args":["route","add","203.0.113.7","via","192.168.4.1"]},
     "undo":{"path":"ip","args":["route","del","203.0.113.7","via","192.168.4.1"]}}
    {"seq":2,"phase":"done","t":"..."}

The uninstaller's replay follows the rules in `LoadJournal` and
`Entry.NeedsUndo` in `internal/netcfg/journal.go`, and any change there has to
be mirrored in the replay:

- the `begin` record carries `op`, `why`, `do` and `undo`. Later records for the
  same `seq` only move the phase on.
- an entry still needs undoing unless its last phase is `undone`. `begin`,
  `done` and `failed` all still need it, because a command killed halfway can
  have landed part of its effect.
- an entry with neither a `do` nor an `undo` is dropped.
- inverses are replayed newest first.

Four properties of the replay are deliberate:

- **It only ever runs `ip`, `iw`, `nft` or `sysctl`.** That is the allowlist in
  `internal/netcfg/command.go`, which exists so the privileged side never runs a
  command built from user input. The same reasoning applies with more force to a
  file that has been sitting on disk. The whole file is checked before anything
  runs, so a journal whose ninth entry names something else does not get its
  first eight executed.
- **It does not print the arguments.** One inverse removes the pinned host route
  to the user's proxy server, so its argument vector contains that server's
  address, and `docs/LAYOUT.md` says the config is never printed or logged. The
  default output is the sequence number and the operation, both of which come
  from the fixed vocabulary in `internal/netcfg/route.go`. `--show-commands`
  prints the rest, and says in `--help` that it does.
- **A line it cannot read is skipped, counted and reported.** That matches
  `LoadJournal`, which drops a truncated tail rather than throwing away every
  complete record before it. Skipped lines make the replay partial, so the
  closing message says the network was not fully restored.
- **A failing inverse does not stop the ones after it**, matching
  `Applier.Teardown`. One inverse usually fails because the thing it undoes is
  already gone.

If the uninstaller refuses a journal outright, it removes nothing and replays
nothing. The software stays installed and the journal stays where it is, so
somebody can look at the problem. `--force` overrides that, removes the
software, and says clearly that the network is not being restored.

The replay is written in Python because the entries are JSON, a shell cannot
parse JSON without help, and the argument vector has to reach `execve` without
passing through a shell at any point. `python3` is therefore required to replay
a journal. Without it the uninstaller refuses rather than guessing.

## Environment variables

All are for testing and for release plumbing. None is needed for a normal
install.

| Variable | Default | What it does |
|---|---|---|
| `CASPIAN_ORG` | `Iman` | Repository owner. Downloads are refused if it is empty |
| `CASPIAN_REPO` | `caspian-byoc` | Repository name |
| `CASPIAN_VERSION` | `latest` | A release tag, or `latest` |
| `CASPIAN_BASE_URL` | derived | Release directory holding the artefact and `SHA256SUMS`. Overrides the three above |
| `CASPIAN_CHECKSUMS_NAME` | `SHA256SUMS` | Name of the checksums file in that directory |
| `CASPIAN_LOCAL_BINARY` | empty | Install from a file on this machine instead of downloading |
| `CASPIAN_LOCAL_CHECKSUMS` | empty | Verify that local file against this checksums file. Without it the installer warns that it is installing unverified |
| `CASPIAN_SCRIPT_BASE_URL` | derived | Where to fetch `uninstall.sh` from |
| `CASPIAN_UNINSTALL_SRC` | empty | Use a local `uninstall.sh` instead of fetching one |
| `CASPIAN_ALLOW_INSECURE_URL` | `0` | Permit a plaintext base URL. Testing only |
| `CASPIAN_SYSROOT` | empty | Prefix every destination path. Refused unless `--dry-run` is also given |
| `CASPIAN_ASSUME_YES` | `0` | Same as `--yes` |
| `CASPIAN_SOURCE_ONLY` | `0` | Makes `main` return immediately, so the test harness can call one function at a time |

## Testing it without a release

### The dry run

    bash install.sh --dry-run

Prints every action it would take and takes none. Every change to the machine
goes through one of two functions, `run` and `write_file`, and both of them
print instead of acting when `--dry-run` is set, which is what makes the dry run
complete rather than approximate.

One thing does happen for real under `--dry-run`: with `CASPIAN_LOCAL_BINARY`
set, the file is copied into a private temporary directory and its checksum is
verified. That touches nothing outside that directory, and running it for real
is the only way a dry run can prove the verification works before a release
exists to test against.

### A fake machine

The refusals and the architecture mapping are decided by `uname`, so they are
tested by putting a `uname` of one's own at the front of `PATH`. Combined with
`CASPIAN_SYSROOT`, a whole install can be walked on a machine that is not Linux:

    mkdir -p /tmp/fake/bin /tmp/fake/sysroot/run/systemd/system
    printf '%s\n' '#!/bin/sh' \
      'case "${1:-}" in' '  -s) echo Linux ;;' '  -m) echo armv6l ;;' 'esac' \
      > /tmp/fake/bin/uname
    printf '%s\n' '#!/bin/sh' \
      'if [ "${1:-}" = "--version" ]; then echo "systemd 252 (252.1)"; exit 0; fi' \
      'exit 0' > /tmp/fake/bin/systemctl
    chmod 0755 /tmp/fake/bin/*

    env PATH=/tmp/fake/bin:$PATH CASPIAN_SYSROOT=/tmp/fake/sysroot \
      CASPIAN_BASE_URL=https://example.invalid/rel \
      bash install.sh --dry-run --yes

### Verifying against a local build

    go build -o /tmp/caspian-linux-arm64 ./cmd/caspian
    sha256sum /tmp/caspian-linux-arm64 | sed 's|/tmp/||' > /tmp/SHA256SUMS

    env CASPIAN_LOCAL_BINARY=/tmp/caspian-linux-arm64 \
        CASPIAN_LOCAL_CHECKSUMS=/tmp/SHA256SUMS \
        bash install.sh --dry-run --yes

Change one byte of the checksums file and the same command refuses, prints both
hashes, and says nothing has been changed.

### The test suite

    bash packaging/test-install.sh

It runs on any machine with bash, including one that cannot be installed to. It
covers:

- `bash -n` and `shellcheck` on both scripts.
- that no shipped file contains an escape code, an emoji or an em dash.
- that the units embedded in `install.sh` still match `packaging/` byte for
  byte.
- the four architecture mappings, including that `armv6l` does not get an armv7
  artefact.
- six refusal paths.
- the four checksum outcomes.
- the shape of the generated password.
- the journal replay, including that a binary off the allowlist is refused and
  never executed, and that the server address is not printed.
- a full dry run for both a fresh install and an upgrade.

### What these tests do not cover

Everything below needs a Raspberry Pi and none of it has been run:

- Whether the hardened `caspian.service` actually starts, and whether
  `SystemCallFilter=@system-service`, `RestrictAddressFamilies` or
  `DevicePolicy=closed` block something that is genuinely needed. The list of
  needed capabilities was reasoned from the code, not measured.
- Whether on-demand nftables module autoload works inside the unit on a fresh
  boot, and whether `/etc/modules-load.d/caspian.conf` covers enough of it.
- Whether the package names are right on anything other than Debian.
- Whether `systemd-tmpfiles --create` produces `/run/caspian` as
  `root:caspian 0750` on the target.
- Whether a real `sha256sum` on the Pi and the checksums file produced by the
  release pipeline agree in format.
- Anything about the panel: nothing consumes the first-run password yet, and the
  printed address has not been shown to be one the panel answers on.
- Whether a journal written by a real run replays cleanly. The replay was
  written against `internal/netcfg/journal.go` and is tested against fixtures in
  that shape, but no journal produced by the actual `Applier` on a real box has
  been through it.

## Before the first release

Four things had to be settled before the one-line install could work. Three are
settled in the release workflow; the fourth was settled in the code.

1. **The repository owner.** `Iman`. It is the default of `CASPIAN_ORG` in
   `install.sh` and it is what the release URLs resolve to.
2. **`GOARM=6` for the `linux/arm` artefact.** The workflow builds it that way
   and then checks the result with `readelf`, failing the release if the
   artefact is not ARMv6. Both `armv6l` and `armv7l` machines install that one
   file, so an ARMv7 build would leave every Pi 1, Zero and Zero W dying with an
   illegal instruction on first run. That has happened once in this workspace
   already, which is why the check exists rather than a comment asking for care.
3. **The checksums file name.** `SHA256SUMS`, in `sha256sum` format, published
   beside the artefacts. It is this installer's choice, so the workflow is
   written to match `CASPIAN_CHECKSUMS_NAME` rather than the other way round.
4. **The first-run password handoff.** Done: `cmd/caspian/firstrun.go` provides
   `consumeFirstRunPassword`, `cmd/caspian/serve_panel.go` calls it at panel
   start, and `cmd/caspian/firstrun_test.go` covers it. The printed password
   works.

What is still true: no release has been cut yet. Until a version tag is pushed,
`releases/latest` resolves to nothing and the one-line install has nothing to
fetch.

