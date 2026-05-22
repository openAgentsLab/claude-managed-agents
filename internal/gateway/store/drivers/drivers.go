// Package drivers registers all built-in app store drivers.
// Import this package with a blank identifier to activate the drivers:
//
//	import _ "forge/internal/gateway/store/drivers"
package drivers

import (
	_ "forge/internal/gateway/store/postgres"
	_ "forge/internal/gateway/store/sqlite"
)
