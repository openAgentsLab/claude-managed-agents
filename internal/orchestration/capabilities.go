package orchestration

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"forge/internal/config"
)

// Capabilities describes which optional system features are available at runtime.
// Populated once at startup via detectCapabilities; stored on HTTPOrchestrator.
type Capabilities struct {
	// SharedStorage is true when VolumesRoot is configured and writable by this
	// process. Enables: git clone in orchestration (no token in sandbox),
	// dynamic file mounts, output directory sharing between service and containers.
	SharedStorage bool `json:"shared_storage"`
}

// detectCapabilities probes the runtime environment to determine which optional
// features are available. Logs a warning for each capability that cannot be
// enabled so operators know what to fix.
func detectCapabilities(cfg config.SandboxConfig) Capabilities {
	var caps Capabilities

	if cfg.VolumesRoot != "" {
		if err := probeWritable(cfg.VolumesRoot); err != nil {
			slog.Warn("shared storage unavailable: volumes_root not writable",
				"path", cfg.VolumesRoot,
				"error", err,
			)
		} else {
			caps.SharedStorage = true
		}
	}

	return caps
}

func (o *HTTPOrchestrator) handleCapabilities(c *gin.Context) {
	c.JSON(http.StatusOK, o.caps)
}

// probeWritable checks that dir exists and this process can create files in it.
func probeWritable(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, ".forge_probe")
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	f.Close()
	_ = os.Remove(tmp)
	return nil
}
