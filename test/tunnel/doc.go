// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Copyright (C) 2026 Iman Samizadeh
//
// This file is part of Caspian-BYOC.

// Package tunnel holds the carriage suite: the tests that prove a share link
// of each supported protocol actually MOVES BYTES through a real xray-core
// server, rather than merely parsing into a document the engine will accept.
//
// # Why it exists
//
// internal/link accepts seven schemes: vless, vmess, ss, socks, trojan,
// hysteria2 and hy2. Before this package, the strongest thing said about any of
// them was that the engine would LOAD the document they produce. Two tests said
// it, and neither says more than that:
//
//	internal/link/integration_test.go, TestEmittedConfigPassesEngineValidate
//	    puts nine links through engine.Validate. They cover five of the seven
//	    schemes; socks and hy2 are not in its case list.
//
//	internal/xcfg/golden_test.go, TestGoldenFilesAreAcceptedByTheEngine
//	    puts every committed golden document through engine.Validate, and
//	    internal/xcfg/testdata does hold a file for all seven schemes.
//
// engine.Validate does not call core.New, so no feature is constructed, no
// inbound handler is created and no socket is opened (internal/engine/engine.go,
// Validate, and the assertion in TestValidateOpensNoListener). Nothing anywhere
// asserted that any of these protocols carries a packet. "The engine would load
// this" and "this tunnel works" are different claims, and only the first one was
// held.
//
// The gap is the shape of failure this project has been bitten by: a test that
// passes because both sides agree by construction, while the thing that has to
// work was never exercised.
//
// # What it drives
//
// Everything is in one process, on loopback, with no root, no Raspberry Pi, no
// TUN device and no dependency on anybody's live infrastructure.
//
//	the client   the product path, unmodified: link.Parse, then xcfg.Build,
//	             then engine.Engine.Start. No config is hand-written; the whole
//	             client document is derived from the share link exactly as the
//	             appliance derives it.
//	the server   a real xray-core instance built from this module's own
//	             xray-core dependency, one protocol at a time, listening on
//	             127.0.0.1. Not a test double: it is core.New over a config
//	             loaded by infra/conf/serial, the same loader internal/engine
//	             uses.
//	the origin   a plain net/http server on 127.0.0.1 returning a token that is
//	             freshly generated for each subtest.
//
// # How a request that skipped the tunnel is stopped from passing
//
// This is the part that decides whether the suite is worth anything, so it is
// stated in full. Four independent controls, all of them executed rather than
// asserted in prose:
//
//  1. The client is never told where the origin is. It is given the name
//     "origin.invalid" and the port of the BYPASS SENTINEL, a second HTTP
//     server that returns a different body. The origin listens on a third,
//     randomly assigned port that appears nowhere the client can reach. The
//     only thing that turns the decoy endpoint into the origin is the server's
//     freedom outbound "redirect", which is server-side configuration.
//
//  2. The name cannot be resolved. ".invalid" is reserved by RFC 6761 section
//     6.4 precisely so that it never resolves, and nothing in the tunnelled
//     path resolves it either: the SOCKS driver sends it as a SOCKS5
//     DOMAINNAME, the client document's routing domainStrategy is AsIs so the
//     router does not resolve it (internal/xcfg/build.go, assemble), and the
//     server's freedom outbound overrides the destination before the only
//     lookup it could make (proxy/freedom/freedom.go, Handler.Process applies
//     DestinationOverride above the LookupForIP block, which its AsIs strategy
//     skips anyway). This is the one control that depends on the machine the
//     test runs on rather than on this code, so it is measured rather than
//     assumed: TestTheOriginIsUnreachableWithoutTheTunnel says so out loud when
//     a resolver on this machine answers .invalid, and every subtest logs it.
//
//  3. The origin checks WHERE THE REQUEST WAS ADDRESSED, not only that it
//     arrived. A request that reached the origin through the server carries
//     "Host: origin.invalid:<decoy port>", because "redirect" rewrites the TCP
//     destination and not the payload. A request that reached the origin any
//     other way carries the origin's own authority. The suite asserts the
//     former, and TestTheProofRejectsARequestThatDidNotGoThroughTheTunnel
//     proves that assertion rejects the latter. This is the control that still
//     holds when control 2 does not.
//
//  4. The bypass sentinel counts its hits. Each subtest requires the TUNNELLED
//     request to add none, counting from after its own control request, so a
//     resolver that answered "origin.invalid" with a loopback address is a loud
//     failure rather than a quiet one, and the suite's own control cannot be
//     mistaken for the thing it is controlling for.
//
// # What this package proves, and what it does not
//
// It proves that a pasted share link of each named protocol produces a running
// tunnel that carries an HTTP request to a server that only the far side of
// that tunnel can reach. It says nothing about the internet, about censorship,
// about a real provider's server, or about the appliance's hotspot, firewall
// and TUN paths, which are test/bdd's and internal/netcfg's. It captures no
// exit IP and cannot: everything here is loopback. Read docs/BEHAVIOUR.md for
// the list of behaviours that are proven and the list that are not.
//
// # Protocol coverage, and the shape of what is NOT covered
//
// vmess, shadowsocks, socks, trojan, hysteria2 and the hy2 alias each get a
// full carriage test. vless is covered too, so the one protocol that was known
// to work is held by the same evidence as the six that were not, and
// TestEveryProtocolTheParserAcceptsIsDrivenEndToEnd reads the accepted-scheme
// list out of internal/link's source so an eighth scheme cannot be added
// without a row here.
//
// Each row proves ONE protocol over ONE transport with ONE set of parameters.
// Read that narrowly, because the combinations this suite does not touch are
// where a share link is most likely to differ from these:
//
//   - transport. Every row but hysteria2 runs over raw TCP. websocket, gRPC,
//     httpupgrade, splithttp and kcp carry no traffic here, and internal/xcfg's
//     goldens are still the only thing that looks at them.
//   - TLS and REALITY. vless, vmess, shadowsocks and socks are driven with
//     security none; trojan and hysteria2 with TLS pinned by digest. No row
//     drives REALITY, whose server side needs a real handshake target.
//   - shadowsocks ciphers. aes-256-gcm only. The 2022 ciphers take a different
//     code path (infra/conf/shadowsocks.go, buildShadowsocks2022) and are not
//     exercised.
//   - hysteria2 without alpn. See hysteriaShareLink for what was measured.
//   - UDP. Every row carries a TCP request. The SOCKS inbound's UDP associate
//     is off, which is xcfg's default.
package tunnel
