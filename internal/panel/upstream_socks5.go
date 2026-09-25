// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import "strings"

// validateUpstreamSOCKS5 validates the user-facing upstream fields. Empty
// credentials mean no-auth; otherwise both username and password are required.
func validateUpstreamSOCKS5(enabled bool, address string, port uint16, username, password string) Problem {
	if !enabled {
		return Problem{}
	}
	if address == "" || port == 0 {
		return Problem{Headline: Key("advanced.upstream.bad")}
	}
	if strings.TrimSpace(address) != address || strings.IndexAny(address, " \t\r\n\x00") >= 0 {
		return Problem{Headline: Key("advanced.upstream.bad")}
	}
	if (username == "") != (password == "") {
		return Problem{Headline: Key("advanced.upstream.authbad")}
	}
	return Problem{}
}
