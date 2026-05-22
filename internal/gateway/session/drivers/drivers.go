// Package drivers activates all built-in SessionStore drivers by triggering
// their init() registration.  Add new session drivers here — main.go never
// needs to change.
//
// Import with a blank identifier from the binary entry point:
//
//	import _ "forge/internal/gateway/session/drivers"
package drivers

import (
	_ "forge/internal/gateway/session/memory"
	_ "forge/internal/gateway/session/postgres"
	_ "forge/internal/gateway/session/sqlite"
)
