package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"forge/internal/config"
	"forge/internal/hands"
	"forge/internal/hands/remote"
	"forge/internal/reqctx"
	"forge/internal/resources"
)

func (p *DockerPool) dockerNetwork(networking resources.NetworkConfig) string {
	if networking.Mode == resources.NetworkingLimited {
		return "none"
	}
	return p.network
}

// start creates a per-session container, binding the session workspace directory
// to /workspace inside the container.
//
// When env.Packages is non-empty the entrypoint runs "forge sandbox-init" before
// starting the tool-server, installing the declared packages into the persistent
// workspace volume so they survive container rebuilds.
func (p *DockerPool) start(ctx context.Context, sessionID, token string, quota hands.ResourceQuota, env resources.Environment) (hands.Sandbox, string, error) {
	if !p.sharedStorageAvailable() {
		return nil, "", hands.ErrSharedStorageUnavailable
	}
	hostPath := p.volumeDir(sessionID, reqctx.TenantIDFromContext(ctx))
	if err := os.MkdirAll(hostPath, 0o755); err != nil {
		return nil, "", fmt.Errorf("create workspace dir %s: %w", hostPath, err)
	}

	args := []string{
		"run", "-d", "--rm",
		"--network", p.dockerNetwork(env.Networking),
		"-p", "127.0.0.1:0:" + toolServerContainerPort,
		"-e", "TOOL_SERVER_TOKEN=" + token,
		"-v", hostPath + ":" + config.ContainerWorkspaceRoot,
		"-w", config.ContainerWorkspaceRoot,
	}

	if quota.MemoryBytes > 0 {
		args = append(args, "--memory", fmt.Sprintf("%d", quota.MemoryBytes))
	}
	if quota.NanoCPUs > 0 {
		cpus := float64(quota.NanoCPUs) / 1e9
		args = append(args, "--cpus", fmt.Sprintf("%.3f", cpus))
	}

	if p.forgeBin != "" {
		args = append(args, "-v", p.forgeBin+":"+forgeBinDest+":ro")
	}

	// Inject user-defined environment variables, skipping reserved system vars.
	for k, v := range env.Env {
		if reservedEnvVar(k) {
			continue
		}
		args = append(args, "-e", k+"="+v)
	}

	// Always wrap in sh -c so env.sh (written by sandbox-init) is sourced
	// before the tool-server starts, making installed packages visible to
	// every tool invocation that runs inside the container.
	envSource := ". " + config.ContainerWorkspaceRoot + "/.forge-env/env.sh 2>/dev/null || true"
	var entrypoint string
	if !env.Packages.IsEmpty() {
		packagesJSON, _ := json.Marshal(env.Packages)
		args = append(args, "-e", "FORGE_PACKAGES_SPEC="+string(packagesJSON))
		entrypoint = forgeBinDest + " sandbox-init --workspace " + config.ContainerWorkspaceRoot +
			" && { " + envSource + "; } && exec " + forgeBinDest + " tool-server --addr :" + toolServerContainerPort
	} else {
		entrypoint = "{ " + envSource + "; }; exec " + forgeBinDest + " tool-server --addr :" + toolServerContainerPort
	}
	args = append(args, p.image, "sh", "-c", entrypoint)

	out, err := exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		return nil, "", fmt.Errorf("docker run for session %q: %w", sessionID, err)
	}
	containerID := strings.TrimSpace(string(out))

	port, err := resolveHostPort(ctx, containerID)
	if err != nil {
		_ = exec.Command("docker", "rm", "-f", containerID).Run()
		return nil, "", fmt.Errorf("resolve host port for session %q: %w", sessionID, err)
	}
	endpoint := "http://localhost:" + port

	if err := waitForEndpoint(ctx, endpoint+"/health"); err != nil {
		_ = exec.Command("docker", "rm", "-f", containerID).Run()
		return nil, "", fmt.Errorf("tool-server not ready for session %q: %w", sessionID, err)
	}

	return remote.New(endpoint, token), containerID, nil
}

func (p *DockerPool) healthy(_ context.Context, sb hands.Sandbox) bool {
	h, ok := sb.(hands.HealthEndpointer)
	if !ok {
		return true
	}
	client := &http.Client{Timeout: healthCheckTimeout}
	resp, err := client.Get(h.HealthEndpoint() + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
