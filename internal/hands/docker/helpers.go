package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func resolveHostPort(ctx context.Context, containerID string) (string, error) {
	out, err := exec.CommandContext(ctx, "docker", "port", containerID, toolServerContainerPort).Output()
	if err != nil {
		return "", fmt.Errorf("docker port: %w", err)
	}
	line := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	_, port, err := net.SplitHostPort(strings.TrimSpace(line))
	if err != nil {
		return "", fmt.Errorf("parse port output %q: %w", line, err)
	}
	return port, nil
}

func waitForEndpoint(ctx context.Context, url string) error {
	client := &http.Client{Timeout: healthCheckTimeout}
	for i := 0; i < readinessAttempts; i++ {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(readinessInterval):
		}
	}
	return fmt.Errorf("endpoint %s not ready after %d attempts", url, readinessAttempts)
}

func optOr(opts map[string]string, key, fallback string) string {
	if v := opts[key]; v != "" {
		return v
	}
	return fallback
}

func resolveForgeBin(opts map[string]string) string {
	if v := opts["forge_bin"]; v != "" {
		return v
	}
	bin, _ := os.Executable()
	return bin
}

func generateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// reservedEnvVar reports whether name is a system env var that must not be
// overridden by user-defined environment maps.
func reservedEnvVar(name string) bool {
	switch name {
	case "TOOL_SERVER_TOKEN", "FORGE_PACKAGES_SPEC":
		return true
	}
	return false
}
