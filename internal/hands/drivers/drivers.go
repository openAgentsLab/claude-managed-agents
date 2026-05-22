// Package drivers activates all built-in Sandbox drivers by triggering
// their init() registration.  Add new sandbox drivers here — main.go never
// needs to change.
//
// Import with a blank identifier from the binary entry point:
//
//	import _ "forge/internal/hands/drivers"
package drivers

import (
	_ "forge/internal/hands/docker"
	_ "forge/internal/hands/k8s"
	_ "forge/internal/hands/local"
)
