// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package link

import "errors"

// The errors in this package are written to be shown to a non-technical user
// and to be safe to write to a log file.
//
// Why they never carry the offending value: the material this package handles
// is exactly the material that must not be logged, and the layers below format
// it into their errors. Read in the source rather than assumed:
//
//   - github.com/xtls/xray-core/infra/conf/transport_internet.go:944 formats
//     `invalid "password": ` with the REALITY public key, and :958 formats
//     `invalid "mldsa65Verify": ` with the post-quantum key. These reach a
//     caller of Build.
//   - third_party/libxray-share/parse_share.go:169 formats "unsupported
//     shadowsocks link password: %s" with the DECODED userinfo, and :290 does
//     the same for socks. Today these do not reach a caller, because
//     parsePlainShareLines drops every per-line error at parse_share.go:104-106
//     and reports only "no valid outbound found". That is a property of the
//     vendored code as it stands, not a promise, and it is one edit away from
//     changing.
//   - what does reach a caller from the parser is a base64 or YAML error that
//     quotes the start of the pasted text.
//
// So this package never wraps an error raised below it. It classifies the
// failure and states it in its own words. That loses some diagnostic detail on
// purpose; the alternative is a panel that prints the user's key when they
// mistype it.
var (
	// ErrEmpty means nothing usable was pasted.
	//
	// This has to be checked here rather than relied on from below: the
	// vendored parser returns (config with zero outbounds, nil error) for
	// empty input. Measured 2026-08-30, and the path is
	// ConvertShareLinksToXrayJson -> tryParseEncodedOrClash ->
	// tryToParseClashYaml, where yaml.Unmarshal of "" into ClashYaml
	// succeeds with no proxies (third_party/libxray-share/clash_meta.go:134-152).
	ErrEmpty = errors.New("nothing was pasted")

	// ErrUnsupportedScheme means the link type is not one this box can use.
	ErrUnsupportedScheme = errors.New("this kind of link is not supported")

	// ErrNoLink means the text was not empty but held no usable link.
	ErrNoLink = errors.New("nothing in the pasted text was a proxy link this box understands")

	// ErrBadUUID means the user id is not a well-formed UUID. See validate.go
	// for why this must be checked here and cannot be left to the engine.
	ErrBadUUID = errors.New("the id in this link is not a valid UUID")

	// ErrBadAddress means the server address was missing or unusable.
	ErrBadAddress = errors.New("the link has no server address")

	// ErrBadPort means the server port was missing or zero.
	ErrBadPort = errors.New("the link has no usable server port")

	// ErrBadReality means a REALITY parameter is present but malformed.
	ErrBadReality = errors.New("a REALITY setting in this link is malformed")

	// ErrUnsupportedTransport means the transport named by the link is one the
	// engine no longer carries.
	ErrUnsupportedTransport = errors.New("this link uses a transport the engine no longer supports")
)
