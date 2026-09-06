# Protocols and transports

[English](https://github.com/Iman/caspian/wiki/Protocols-and-Transports) | [فارسی](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa) | [Русский](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru) | [中文](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh)

[Caspian wiki](https://github.com/Iman/caspian/wiki/Home)

> This guide comes from the existing README. Its measurements retain their original dates; this documentation move does not report a new test run.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## What you can paste, and what it will refuse

You bring the config. This is what the box accepts, taken from the code that
does the accepting rather than from a wish list. Every row was measured against
`internal/link` and the pinned engine.

| | It works | It is refused |
|---|---|---|
| Share links | `vless://` `vmess://` `ss://` `socks://` `trojan://` `hysteria2://` `hy2://` | `tuic://` `ssr://` `wireguard://` `anytls://` `naive+https://` `hysteria://` (version 1) |
| Pasted documents | Clash and Clash.Meta YAML, raw xray JSON, a list of links one per line, a base64 subscription blob | a subscription URL, a base64-wrapped Clash document, a JSON array, text whose first line is a comment |
| Transports | `raw` (also written `tcp`), `ws`, `grpc`, `httpupgrade`, `xhttp` (also `splithttp`), `kcp` and `mkcp` | `h2`, `h3`, `http`, `quic`, `gun` |
| Security | `none`, `tls`, `reality` | `xtls` (the legacy kind), `allowInsecure` |
| VLESS flow | `xtls-rprx-vision`, `xtls-rprx-vision-udp443`, or none | every other value |

`h2` and `h3` in that refused column are transport NAMES. HTTP/2 and HTTP/3
themselves are carried: `type=xhttp` with `security=tls`, and the TLS ALPN
decides which. See [HTTP/2 and HTTP/3 are carried, under a different
name](https://github.com/Iman/caspian/wiki/Protocols-and-Transports#http2-and-http3-are-carried-under-a-different-name).

Six things surprise people, so they are here rather than in a footnote:

Only the FIRST link is used. Paste forty servers and you configure one; the
panel tells you how many it found. `ss://` and `socks://` need the base64 form
of their user information, and the plain `method:password@host` spelling is
refused. REALITY works over `raw`, `xhttp` and `grpc` only, so pairing it with
WebSocket is refused by the engine at paste time rather than failing later.
`security=` has to be lowercase here even though the engine itself does not
care, and an uppercase `TLS` is reported back to you as `none`. A `plugin=`
parameter on an `ss://` link is ignored without saying so. And a subscription
URL is refused because the panel fetches nothing from the internet, which is a
deliberate property rather than a missing feature.

The full picture, including which of these have carried real bytes and which
have been proven end to end on hardware with an exit address captured, is under
[Protocols and transports](https://github.com/Iman/caspian/wiki/Protocols-and-Transports#protocols-and-transports). Those are three different
claims and this project does not let them blur.

## Protocols and transports

A share link carries three separate things, and it helps to keep them apart: the
proxy protocol, the transport that carries it, and the encryption layer wrapped
around that transport. A VLESS link over WebSocket with TLS and a VLESS link
over plain TCP with REALITY are the same protocol reaching the same kind of
server by two different routes. They fail in different ways.

The proxy protocols are the seven schemes listed above. VLESS is the one most of
this document's examples use, because it is what REALITY is built for. Nothing
in the appliance is specific to it. The parser produces a description,
`internal/xcfg` composes an engine document around it, and the rest of the box
does not know which protocol it is carrying.

The transports come from xray-core and are named in the vendored parser:

- `tcp`, also written `raw`
- `ws`, for WebSocket
- `httpupgrade`
- `xhttp`, the protocol formerly called SplitHTTP. Both spellings parse
- `grpc`
- `kcp` and `mkcp`, for mKCP

`h2`, `http`, `h3` and `quic` are not on that list. The engine version this pins
removed them, so a link asking for one is refused rather than carried, and
`TestRemovedTransportsAreRefusedWithASentence` in `internal/link` holds that
refusal in place.

How well the refusal reads depends on the route in. A Clash document naming one
gets a sentence about the transport. The same transport in a `type=` parameter
on a share link arrives as the generic "nothing in the pasted text was a proxy
link this box understands", which is correct and unhelpful.
`TestRemovedTransportInAURIIsReportedLessWell` pins that difference so it is a
known gap rather than a surprise.

### HTTP/2 and HTTP/3 are carried, under a different name

Being refused `type=h2` or `type=quic` does not mean the box cannot speak them.
It means the spelling moved. XHTTP replaced both, and it chooses its HTTP
version from the TLS ALPN rather than from the transport name:

| What you want | What to write |
|---|---|
| HTTP/3, which is QUIC | `type=xhttp` with `security=tls`, `alpn=h3` and `mode=stream-one` |
| HTTP/2 | `type=xhttp` with `security=tls` and any ALPN that is not exactly `h3` |
| QUIC, without XHTTP | a `hysteria2://` link, which is QUIC underneath and needs `alpn=h3` |

The keys reach the engine untouched: `internal/xcfg` carries the outbound as
opaque JSON and never decodes it, so `alpn`, `mode`, `xmux` and the QUIC tuning
block arrive exactly as pasted.

Four details decide whether you get h3 or silently get something else:

`alpn` must be exactly one value and that value must be `h3`. Writing
`alpn=h3,h2` gives you HTTP/2 with no warning, because the engine takes a list
of any other length as a request for version 2. REALITY forces HTTP/2 whenever
it is present, so REALITY and h3 are mutually exclusive and pairing them gets
you h2 rather than an error. `mode` has to be set explicitly, because the
default resolves to `packet-up` rather than the `stream-one` shape the engine
names as the QUIC replacement. And `downloadSettings`, for a split upload and
download, is refused together with `mode: stream-one`; that combination needs
`stream-up`.

One collision in vocabulary is worth stating plainly, because it reads like a
contradiction: `type=h3` is refused, and `alpn=h3` is required. They are
different fields. The first names a transport that no longer exists; the second
names the protocol negotiated inside TLS.

These configurations are accepted and validated by the box. They have not yet
been driven against a live server from here, so treat the row as the engine's
capability rather than as something this project has watched work.

The security layer is `reality`, `tls`, or `none`.

Not every combination is equally useful. REALITY is normally paired with plain
TCP, because its whole method is to borrow a real site's TLS handshake, so
wrapping it in another TLS layer defeats the point. WebSocket, HTTPUpgrade and
XHTTP exist to look like ordinary web traffic to something inspecting the
connection, and they are usually paired with TLS for the same reason an ordinary
website is. WebSocket with `security=none` is the one shape to think twice
about. It is plaintext on the wire, and it is only sensible when something else
already provides the encryption, such as a CDN terminating TLS in front of the
server.

### Three different claims, kept apart

The distinction below is the most important thing in this document. Read the
column headings before the rows.

| Claim | What it rests on | What it is worth |
|---|---|---|
| The parser accepts it | `internal/link`, and a committed golden engine document | The document is stable. Nothing was dialled |
| It carries bytes | `test/tunnel`, a real xray-core server on loopback | Traffic moved through the protocol. No exit IP, no appliance, no internet |
| It is proven end to end | `test/hardware`, a real phone on the hotspot | Real traffic left the box and the exit address was captured and named |

### What has carried bytes through a real server

Added by `test/tunnel`. Every scheme the parser accepts is driven end to end
against a real xray-core instance, built from this module's own dependency and
loaded through the same loader `internal/engine` uses. The client side is the
product path, unmodified: `link.Parse`, then `xcfg.Build`, then
`engine.Engine.Start`. No config is hand-written.

| protocol | transport | security | carries an HTTP request |
|---|---|---|---|
| VLESS | tcp (raw) | none | yes |
| VMess | tcp (raw) | none | yes |
| Shadowsocks, aes-256-gcm | tcp (raw) | none | yes |
| SOCKS | tcp (raw) | none | yes |
| Trojan | tcp (raw) | TLS, pinned by digest | yes |
| Hysteria2, and the `hy2` alias | QUIC | TLS, pinned by digest | yes |

Four controls stop a request that skipped the tunnel from passing, and all four
run rather than being asserted in prose. The client is never told where the
origin is, and is given a `.invalid` name and the port of a decoy. The name
cannot be resolved, and the suite says so out loud if a resolver on the machine
answers it anyway. The origin checks where the request was addressed, not only
that it arrived. The decoy counts its own hits, and a tunnelled request must add
none. `TestEveryCarriageProofCanFail` and
`TestTheProofRejectsARequestThatDidNotGoThroughTheTunnel` are what make those
controls evidence rather than intent.

Read each row narrowly. Every row but Hysteria2 runs over raw TCP. No row drives
REALITY, whose server side needs a real handshake target. Shadowsocks is
aes-256-gcm only, because the 2022 ciphers take a different code path. Every row
carries a TCP request, and UDP associate is off. Everything is on loopback, so
no exit IP is captured and none can be.

`TestEveryProtocolTheParserAcceptsIsDrivenEndToEnd` reads the accepted-scheme
list out of `internal/link`'s source, so an eighth scheme cannot be added
without a row here.

### What has actually been proven on hardware

The table below is what real traffic has traversed with an exit IP captured. It
is not what the parser accepts, and it is not what the loopback suite carries.

| protocol | transport | security | proven end to end |
|---|---|---|---|
| VLESS | tcp (raw) | REALITY | yes, on three separate servers |
| VLESS | ws (WebSocket) | none, plus VLESS Encryption | yes |
| VLESS | ws (WebSocket) | TLS | yes, through a CDN |
| VLESS | httpupgrade | TLS | yes, through a CDN |
| VLESS | xhttp | TLS | yes |
| VMess, Trojan, Shadowsocks, SOCKS, Hysteria2 | any | any | no |

Each of those was proven by driving a real browser on a real phone joined to the
hotspot. The exit address was captured from two independent sources and matched
to the server the configuration names. Three different servers were used and
each returned a different address, so a repeated or cached reading cannot be
mistaken for a working tunnel.

A row that is not proven is not a claim that it is broken. It is a claim that
nobody has watched a packet come out of the far end, which is a different thing
and the only thing this project treats as evidence. The engine document each
transport produces IS pinned as a golden file, so a change to how one is
composed shows up as a diff. That proves the document is stable and says nothing
about whether the transport connects.

### Why a row with no transport security is still encrypted

The `security` column above is about the layer WRAPPED AROUND the transport, and
`none` there does not mean "no encryption". It means no TLS and no REALITY. That
is worth being precise about, because reading it the other way would be alarming
and reading it too generously would be worse.

VLESS by itself carries no encryption. It is a stateless protocol that expects
the layer underneath to provide confidentiality, which is normally REALITY or
TLS. A VLESS link over WebSocket with `security=none` and nothing else WOULD be
plaintext on the wire, and the exit address would be proven while every packet
was readable by anything on the path.

What makes that row safe is VLESS Encryption, carried in the link's
`encryption=` parameter. It is a hybrid key exchange, ML-KEM-768 for
post-quantum resistance combined with X25519, applied at the VLESS layer itself
rather than underneath it. So the traffic is encrypted, and it is encrypted by
something designed to stay secure against an attacker who records it today and
has a quantum computer later. A link carrying `encryption=none` AND
`security=none` has neither, and that is the combination to refuse.

This is NOT the Noise Protocol Framework (noiseprotocol.org). Nothing in this
appliance, in the vendored share-link parser, or in the engine implements Noise.
The word "noise" appears in xray-core's configuration for something unrelated,
padding traffic with random bytes to change its shape on the wire, which is
obfuscation and not a handshake. The thing that gives this row its
confidentiality is VLESS Encryption, and the name matters because the two
provide different guarantees.

MEASURED rather than assumed, on 2026-08-30. This package does not rebuild the
outbound field by field. It re-serialises what the parser produced, and the
protocol settings ride along as an opaque blob. That is why the parameter
survives. It is also why nothing would break if it stopped surviving: no field
would be missing, no type would change, and no other test would notice, while
the tunnel carried a user's traffic in the clear with every check still green.
`TestVLESSEncryptionSurvivesIntoTheEngineDocument` in `internal/link` is the
guard, and it was watched failing against exactly that silent downgrade before
it was kept.

### A certificate name that did not match, and the client-side fix

One result is worth recording, because it is a fault this appliance correctly
refuses to paper over. Two configurations pointed at a server's own address
while carrying the TLS name of the CDN in front of it. The engine reported:

    transport/internet/httpupgrade: failed to dial request ...
      tls: failed to verify certificate: x509: certificate is valid for
      <the apex>, not <the cdn subdomain>

That is a certificate that genuinely does not match the name asked for, and
refusing it is the behaviour you want. Accepting it would mean the tunnel could
be terminated by anything holding any certificate.

The cause and the fix are both on the client side, and no server change is
needed. A share link carries two names that people assume have to match and do
not:

    sni   the name TLS validates the certificate against
    host  the name the server routes the request on, an HTTP header

The failing links carried the CDN's name in BOTH. Through the CDN that works,
because the CDN holds a certificate for it. Pointed straight at the origin it
cannot, because the origin holds a certificate for the apex only. Set `sni` to
the name the certificate actually carries, and leave `host` as the name the
server routes on:

    sni=example.com          host=cdn.example.com

MEASURED on 2026-08-30. Two links that had failed with the certificate error
above both connected after that one change. Exit addresses were captured from
two independent sources and matched to their own servers, and the DNS leak and
fail-closed checks passed in the same run.

So if a transport fails only when pointed straight at the origin, compare `sni`
against the origin certificate's subject alternative names before you suspect
the transport. `openssl s_client -connect <address>:443 -servername <name>`
prints what the server actually presents.

### The panel takes a pasted link and not an image

Dropping a QR image is described in the design, section 5.2, and is **not
implemented**. `internal/panel/qr` is an encoder only, and no handler in
`internal/panel` reads a multipart upload. The QR code the panel does produce is
the one a phone scans to join the hotspot. [`internal/panel/view.go`](https://github.com/Iman/caspian/blob/main/internal/panel/view.go) builds it
with `qr.Encode` and `qr.WiFiJoin`, so no image library and no remote service is
involved.

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

[English: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports#http2-and-http3-are-carried-under-a-different-name) | [English](https://github.com/Iman/caspian/wiki/Protocols-and-Transports#protocols-and-transports) | [فارسی: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa#http2-و-http3-حمل-میشوند-با-نامی-دیگر) | [فارسی](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa#پروتکلها-و-ترابریها) | [Русский: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru#http2-и-http3-переносятся-просто-под-другим-именем) | [Русский](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru#протоколы-и-транспорты) | [中文: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh#http2-与-http3-是被承载的只是换了个名字) | [中文](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh#协议与传输)
