// Package sandboxinit handles idempotent package installation inside a sandbox
// container. It is invoked by the "forge sandbox-init" CLI command which runs
// before the tool-server starts when FORGE_PACKAGES_SPEC is set.
package sandboxinit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PackageSpec lists packages to pre-install in a sandbox container.
type PackageSpec struct {
	Pip   []string `json:"pip,omitempty"`
	Npm   []string `json:"npm,omitempty"`
	Apt   []string `json:"apt,omitempty"`
	Cargo []string `json:"cargo,omitempty"`
}

const (
	envForgePackagesSpec = "FORGE_PACKAGES_SPEC"
	forgeEnvDir          = ".forge-env"
	versionFile          = "version"
)

// Run reads FORGE_PACKAGES_SPEC from the environment, computes a hash of the
// declared packages, and installs them into workspaceRoot/.forge-env/ when the
// hash does not match the stored version. Idempotent: already-installed packages
// are skipped on container rebuild.
func Run(workspaceRoot string) error {
	specJSON := os.Getenv(envForgePackagesSpec)
	if specJSON == "" {
		return nil // nothing to install
	}

	var spec PackageSpec
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		return fmt.Errorf("parse FORGE_PACKAGES_SPEC: %w", err)
	}

	if len(spec.Pip) == 0 && len(spec.Npm) == 0 && len(spec.Apt) == 0 && len(spec.Cargo) == 0 {
		return nil
	}

	envDir := filepath.Join(workspaceRoot, forgeEnvDir)
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		return fmt.Errorf("create env dir: %w", err)
	}

	// apt installs to system paths which are wiped on every container rebuild.
	// Always run it regardless of the cached hash.
	if len(spec.Apt) > 0 {
		if err := installApt(spec.Apt); err != nil {
			return err
		}
	}

	// pip, npm, and cargo install into the persistent workspace volume — skip
	// when the spec hash matches the stored version (idempotent across rebuilds).
	if len(spec.Pip) == 0 && len(spec.Npm) == 0 && len(spec.Cargo) == 0 {
		return nil
	}

	// Hash only the persistent-volume portion so that apt-only changes don't
	// invalidate the workspace cache.
	persistentJSON, _ := json.Marshal(struct {
		Pip   []string `json:"pip,omitempty"`
		Npm   []string `json:"npm,omitempty"`
		Cargo []string `json:"cargo,omitempty"`
	}{Pip: spec.Pip, Npm: spec.Npm, Cargo: spec.Cargo})
	hash := specHash(string(persistentJSON))
	versionPath := filepath.Join(envDir, versionFile)

	if stored, err := os.ReadFile(versionPath); err == nil && strings.TrimSpace(string(stored)) == hash {
		return nil // already up to date
	}

	if err := installPipNpm(envDir, spec); err != nil {
		return err
	}

	if len(spec.Cargo) > 0 {
		if err := installCargo(envDir, spec.Cargo); err != nil {
			return err
		}
	}

	if err := writeEnvScript(envDir, spec); err != nil {
		return fmt.Errorf("write env script: %w", err)
	}

	if err := os.WriteFile(versionPath, []byte(hash), 0o644); err != nil {
		return fmt.Errorf("write version file: %w", err)
	}
	return nil
}

func installPipNpm(envDir string, spec PackageSpec) error {
	if len(spec.Pip) > 0 {
		targetDir := filepath.Join(envDir, "python")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return fmt.Errorf("create pip target dir: %w", err)
		}
		args := append([]string{"install", "--target=" + targetDir}, spec.Pip...)
		if err := run("pip", args...); err != nil {
			return fmt.Errorf("pip install: %w", err)
		}
	}

	if len(spec.Npm) > 0 {
		prefixDir := filepath.Join(envDir, "node")
		if err := os.MkdirAll(prefixDir, 0o755); err != nil {
			return fmt.Errorf("create npm prefix dir: %w", err)
		}
		args := append([]string{"install", "--prefix=" + prefixDir}, spec.Npm...)
		if err := run("npm", args...); err != nil {
			return fmt.Errorf("npm install: %w", err)
		}
	}
	return nil
}

func installCargo(envDir string, packages []string) error {
	cargoDir := filepath.Join(envDir, "cargo")
	if err := os.MkdirAll(cargoDir, 0o755); err != nil {
		return fmt.Errorf("create cargo dir: %w", err)
	}
	for _, pkg := range packages {
		if err := run("cargo", "install", "--root="+cargoDir, pkg); err != nil {
			return fmt.Errorf("cargo install %s: %w", pkg, err)
		}
	}
	return nil
}

func installApt(packages []string) error {
	if err := run("apt-get", "update", "-qq"); err != nil {
		return fmt.Errorf("apt-get update: %w", err)
	}
	args := append([]string{"install", "-y", "-qq"}, packages...)
	if err := run("apt-get", args...); err != nil {
		return fmt.Errorf("apt-get install: %w", err)
	}
	return nil
}

// writeEnvScript writes a sourceable shell script that adds the pip/npm
// install directories to PYTHONPATH / NODE_PATH / PATH.
func writeEnvScript(envDir string, spec PackageSpec) error {
	var lines []string
	if len(spec.Pip) > 0 {
		pythonDir := filepath.Join(envDir, "python")
		lines = append(lines, `export PYTHONPATH="`+pythonDir+`${PYTHONPATH:+:$PYTHONPATH}"`)
	}
	if len(spec.Npm) > 0 {
		nodeDir := filepath.Join(envDir, "node")
		lines = append(lines,
			`export NODE_PATH="`+filepath.Join(nodeDir, "node_modules")+`${NODE_PATH:+:$NODE_PATH}"`,
			`export PATH="`+filepath.Join(nodeDir, ".bin")+`:$PATH"`,
		)
	}
	if len(spec.Cargo) > 0 {
		cargoDir := filepath.Join(envDir, "cargo")
		lines = append(lines, `export PATH="`+filepath.Join(cargoDir, "bin")+`:$PATH"`)
	}
	if len(lines) == 0 {
		return nil
	}
	content := "#!/bin/sh\n" + strings.Join(lines, "\n") + "\n"
	return os.WriteFile(filepath.Join(envDir, "env.sh"), []byte(content), 0o755)
}

func specHash(specJSON string) string {
	h := sha256.Sum256([]byte(specJSON))
	return hex.EncodeToString(h[:])
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
