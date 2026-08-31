// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

// Package panel is the web interface, which after the installer has run is the
// entire product surface (docs/2026-08-29-design.md section 5.1).
//
// The person on the other side of it received a config from somebody they trust
// and wants the devices in the room to work. They will not open a terminal,
// read a log or edit a file. Everything in this package follows from that.
//
// # What it serves
//
// Basic mode is one screen: a switch, a status line, the hotspot name and
// password with a QR code so a phone joins by camera, how many devices are
// connected, one control to add a config, and one line naming what was detected
// (section 5.2). Advanced mode is a toggle that reveals what basic mode decided
// and lets it be changed (section 5.3). Server-rendered HTML, no framework, no
// build step, and a single small script that only keeps the status line current.
//
// # The four properties this package exists to hold
//
// It fetches nothing. No CDN, no web font, no remote script or stylesheet, no
// favicon fetch. Every byte the browser loads is compiled in with go:embed. The
// privacy reason is that a remote asset tells a third party the address of
// everyone who opens the panel; the stronger reason is that the panel has to
// load when the tunnel is down, which is exactly when it is needed (section
// 5.7). Enforced three ways: assets.go embeds everything, setSecurityHeaders
// serves a Content-Security-Policy whose every source is 'self', and
// TestNoAssetReferencesAnExternalURL scans the assets and every rendered page.
//
// It holds no privilege. The process runs as the caspian user and asks a
// service running as root, over a unix socket, to act. That interface is the Go
// interface Privileged in priv.go: a fixed set of named actions, typed arguments, and no
// way to express a command, a path or an argument list (section 5.5). This
// package does not implement the privileged side.
//
// It authenticates before anything. A password on the first screen, verified
// through internal/state, which compares in constant time. Sessions are
// server-side records behind an unguessable cookie. Failed attempts are rate
// limited per address and across the panel. A box with no password set shows
// setup rather than a login form nobody could get past.
//
// It treats the pasted config as a credential. It is never rendered back, never
// written to a log line, never put in a URL or a query parameter, and never in
// an error page. What the panel shows about it is the redacted view from
// internal/link, which carries no key material at all. What crosses the
// privilege boundary is a config document that internal/link re-serialised from
// parsed structures, never the text the user pasted (section 6).
//
// # Plain words
//
// Every sentence the panel can show is in words.go. "No adapter on this machine
// can create a hotspot. Plug in a USB WiFi adapter" is right; "no AP-capable
// phy" is not. A config that fails is reported as one of three distinct states,
// because they need three different actions from the user: it did not parse, it
// parsed and the engine rejected it, or it loaded and the server did not answer
// (section 8, step 11).
//
// # What this package deliberately does not do
//
// It does not decode an uploaded QR image, although section 5.2 offers that as
// a second way to add a config. Section 9 names it as a hazard: decoding an
// uploaded image is untrusted image parsing inside the panel process, which is
// the process that holds the session cookie and the state file handle. Adding
// it means adding an image decoder to this attack surface, and the paste path
// covers the same need. If it is ever added it belongs behind the privilege
// boundary or in a separate process, not here.
//
// It generates a QR code rather than depending on a library for one, because
// go.mod carries none; see internal/panel/qr.
//
// It does not implement the privileged service, the teardown journal, or
// anything that touches the network.
package panel
