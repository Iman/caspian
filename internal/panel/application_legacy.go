//go:build !flutterui

// SPDX-License-Identifier: AGPL-3.0-or-later
package panel

import "net/http"

// Application keeps the legacy harness usable without generated Flutter assets.
// Release builds use the flutterui tag and serve only the Flutter application.
func Application(api http.Handler) http.Handler { return api }
