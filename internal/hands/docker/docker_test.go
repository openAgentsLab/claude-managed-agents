package docker

import (
	"os/exec"
	"testing"

	"forge/internal/hands"
)

// dockerAvailable returns true when the docker CLI is on PATH and the daemon
// is reachable.
func dockerAvailable() bool {
	return exec.Command("docker", "info").Run() == nil
}

func TestDockerPool_ImplementsPoolInterface(t *testing.T) {
	var _ hands.Pool = (*DockerPool)(nil)
}
