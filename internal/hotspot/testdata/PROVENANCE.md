# Provenance of the hotspot fixtures

Same discipline as `internal/netcfg/testdata/PROVENANCE.md`. A fixture nobody
can trace is a fixture whose class nobody can tell, so it silently becomes
evidence it is not. Every file here is named below with what produced it and
what a diff in it means.

`test/goldenscan`'s `TestEveryTestdataDirectoryHasAProvenanceFile` requires this
file to exist. It was added on 2026-08-30, after the directory had gone without
one; the entries for the files that predate it are written from the tests that
own them and are marked as such, because reconstructing provenance from a test
comment is second-hand and should read that way.

## Class of every file here

GENERATED or HAND-WRITTEN. Nothing in this directory was captured from a
running system.

- The `.golden` files are the output of `RenderHostapd` and `RenderDnsmasq` in
  this package.
- The `.leases` files were written by hand in dnsmasq's lease-file format
  (`internal/hotspot/leases_test.go` says so at the top of the file, which is
  the record for them).

## What a green run here does and does not prove

This asymmetry is the single most important thing in this directory and it is
easy to assume away.

- **dnsmasq CAN be checked.** `dnsmasq --test --conf-file=X` parses and exits.
  `TestGoldenDnsmasqConfigIsAcceptedByDnsmasq` in `external_test.go` puts the
  committed default file through the real dnsmasq, and
  `TestGoldenVariants_DnsmasqFilesAreAcceptedByDnsmasq` extends that to every
  variant. Measured on the target Raspberry Pi on 2026-08-30, dnsmasq 2.91:
  "dnsmasq: syntax check OK", exit 0. That measurement is recorded in
  `external_test.go` and is quoted here, not re-made.
- **hostapd CANNOT.** hostapd has no validate-and-exit flag. The full option set
  of hostapd 2.10 was read off the box on 2026-08-30 and none of h, d, B, K, t,
  v, P, e, g, G, f, T, i, S checks a configuration without starting it. In
  particular `-t` is NOT a test flag: it adds timestamps to debug messages, and
  anyone reaching for it as a syntax check gets a running access point instead.

So on a developer Mac, after a green run: the dnsmasq files have been checked by
NOTHING (dnsmasq is not installed there and the test skips, loudly), and the
hostapd files have been checked by nothing anywhere except this package's own
assertions about its own output. Every hostapd golden is a change detector and
only a change detector until a bring-up on hardware says otherwise.

## How to regenerate

    bash scripts/golden-update.sh

or this package alone:

    go test ./internal/hotspot -run Golden -update

Then READ THE DIFF.

## Credentials

A hostapd configuration cannot exist without a `wpa_passphrase` line: that line
is the file's purpose. Two different things are done about that here, and the
difference is deliberate.

- The **variant** goldens (`hostapd-*.golden`, added 2026-08-30) REDACT it. The
  value is replaced by a sha256 prefix of itself plus its length, so a change to
  the passphrase is still a diff while the key itself never lands in a commit.
  `TestGoldenVariants_RedactionStillDetectsAPassphraseChange` proves the digest
  is not a constant, which is the failure mode of redaction done badly: the file
  looks safe and detects nothing.
- `hostapd.golden`, which predates them, still carries `correct-horse-battery`
  in the clear. It is an invented value, declared as `testAP().Passphrase` in
  `internal/hotspot/golden_test.go` and also used as `testPassword` in
  `internal/panel/panel_test.go`, and it has never been a working credential.
  It is left alone and allowlisted BY NAME in `test/goldenscan/registry.go`
  because it is the exact byte sequence the on-target dnsmasq and hostapd
  evidence in `external_test.go` is recorded against; rewriting it would
  invalidate that record.

  If that file is ever regenerated for another reason, redacting its passphrase
  at the same time would remove the last cleartext key from this directory and
  the allowlist entry with it. That is a maintainer's call, not this layer's.

The variants use `variantPassphrase`, a sentinel that occurs nowhere else in the
repository, so `test/goldenscan` can report a hit on it with no ambiguity.
`TestGoldenVariants_NoGoldenCarriesTheSentinelPassphrase` scans every `.golden`
in this directory for it and for the default fixture value, and permits the
second only in `hostapd.golden`, by exact name so a new file cannot inherit the
exception by being named similarly.

## Every file

### The default arrangement, predating 2026-08-30

    hostapd.golden

`RenderHostapd(testAP())`. testAP is channel 10 on 2.4 GHz, country GB,
interface wlan0. Channel 10 is not arbitrary: it is the channel the Raspberry
Pi 5 in `docs/2026-08-29-design.md` section 4.6 was measured holding on its
client link, and the built-in radio reports "#channels <= 1", so it is also the
only channel the access point could use on that box. Owned by
`TestRenderHostapdGolden`.

    dnsmasq.golden

`RenderDnsmasq(testDNS())`. Subnet 192.168.66.0/24, gateway .1, pool .50 to
.150, 12 hour leases, upstream 127.0.0.1:5354, cache 1000. Owned by
`TestRenderDnsmasqGolden`, and put through the real dnsmasq by
`TestGoldenDnsmasqConfigIsAcceptedByDnsmasq`.

### The lease-file fixtures, predating 2026-08-30

Written by hand in dnsmasq's lease-file format. The record for these is the
comment at the top of `internal/hotspot/leases_test.go`; this entry is
second-hand and says so.

    dnsmasq.leases            a normal file with several live leases
    dnsmasq.leases.empty      zero bytes: a hotspot nobody has joined
    dnsmasq.leases.ipv6       leases with IPv6 entries, which v1 does not carry
    dnsmasq.leases.malformed  lines the parser must count and skip rather than fail on

#### Their MAC addresses were rewritten on 2026-08-31, and the leases are still
#### hand-written

The addresses in them were invented, and this file has said so since it was
written. But they were invented by taking a REAL manufacturer OUI and putting
an arbitrary tail on it, which is indistinguishable from a real device to a
reader and to any scanner. They now come from `02:00:5e:02:` instead, one per
client, and the DHCP client identifiers and the DHCPv6 DUID were updated to
keep their trailing address equal to the lease's, which is what those formats
carry.

That block has the locally-administered bit set, so it collides with no
manufacturer. Doing this rather than allowlisting the files is what lets
`test/goldenscan` hold NO exception for a MAC address anywhere in the
repository: an exception here would also have permitted the day somebody pasted
a real `dnsmasq.leases` from a running box into the same file. The claim at the
top of this document is unchanged and is still the point: nothing here was
captured from a real network. The device hostnames beside the addresses
(`iPhone`, `Sara-MacBook-Air`, `pixel-8`, `kitchen-printer`, `tablet`) are
hand-written too and were left alone, because they are what makes the fixture
read like a real lease file and none of them is anybody's device.

The substitute table, and what each block of addresses stands for, is in
`internal/netcfg/testdata/PROVENANCE.md`. Use it rather than inventing a
second scheme.

### The hostapd variants, added 2026-08-30

One per setting that changes the file. Passphrase redacted in all of them.
`TestGoldenVariants_EveryVariantActuallyDiffersFromTheDefault` fails on any
variant whose output equals the default's, because such a file proves nothing
and reads in a listing as though it proves something.

    hostapd-default.golden      the default arrangement, for the variants to vary from
    hostapd-band-5ghz.golden    Band, moved to 5 GHz. Channel moves to 36 with it: a
                                2.4 GHz channel number is not valid on 5 GHz, so the
                                band cannot be moved on its own
    hostapd-channel.golden      Channel, moved to 1
    hostapd-country.golden      Country, moved to IR
    hostapd-interface.golden    HotspotInterface, moved to ap0
    hostapd-utf8-ssid.golden    a Persian network name. An SSID is 32 OCTETS, not 32
                                characters, and a Persian name reaches the limit in
                                roughly sixteen. This is the product's primary audience
                                typing its own language into the field
    hostapd-control-dir.golden  the hostapd control socket directory

`TestGoldenVariants_CoverEveryAdvancedSettingThatReachesHostapd` asserts every
field of `APConfig` except the passphrase is moved by some variant, so a field
added later arrives as a failure naming itself rather than as silent
under-coverage.

### The dnsmasq variants, added 2026-08-30

    dnsmasq-default.golden          the default arrangement
    dnsmasq-subnet.golden           Subnet moved to 10.62.0.0/24, with the gateway and
                                    the whole DHCP pool moved with it. A subnet override
                                    that changed the listen address and not the pool
                                    would be a hotspot handing out unroutable addresses
    dnsmasq-interface.golden        HotspotInterface, moved to ap0
    dnsmasq-no-cache.golden         CacheSize 0, which disables the answer cache
    dnsmasq-filter-aaaa.golden      FilterAAAA on. Off by default because filter-AAAA is
                                    a dnsmasq 2.81 addition and an older dnsmasq treats
                                    an unknown option as FATAL, so the hotspot would not
                                    start at all. The difference between "no IPv6
                                    answers" and "no hotspot" is this one line
    dnsmasq-lease-time.golden       LeaseTime, moved to one hour
    dnsmasq-service-account.golden  the account dnsmasq drops to after binding port 53
    dnsmasq-upstream-port.golden    the local resolver this box runs

`TestGoldenVariants_CoverEveryFieldThatReachesDnsmasq` asserts every field of
`DNSConfig` except `LeaseFile` is moved by some variant. `LeaseFile` is excluded
deliberately: it is a path chosen by the installer rather than a setting, and
moving it would change one literal and teach nobody anything. That exclusion is
stated in the test so its absence reads as a decision rather than an omission.
