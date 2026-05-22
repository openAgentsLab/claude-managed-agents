// Package drivers imports all StoreBackend implementations so their init()
// functions register themselves with memory.Register.
package drivers

import (
	_ "forge/internal/memory/stores/postgres"
	_ "forge/internal/memory/stores/sqlite"
)
