package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultTimeoutMs = 30_000
	maxTimeoutMs     = 120_000
	maxOutputBytes   = 256 * 1024
)

func normalizeTimeoutMs(ms int) int {
	if ms <= 0 {
		return defaultTimeoutMs
	}
	if ms > maxTimeoutMs {
		return maxTimeoutMs
	}
	return ms
}

func resolveWorkdir(workspaceRoot, workdir string) (string, error) {
	if strings.TrimSpace(workdir) == "" {
		return filepath.Clean(workspaceRoot), nil
	}
	p := filepath.Clean(workdir)
	if !filepath.IsAbs(p) {
		p = filepath.Join(workspaceRoot, p)
	}
	return filepath.Clean(p), nil
}

func detectUserShell() string {
	if runtime.GOOS == "windows" {
		return "powershell.exe"
	}
	if s := strings.TrimSpace(os.Getenv("SHELL")); s != "" {
		return s
	}
	if runtime.GOOS == "darwin" {
		return "/bin/zsh"
	}
	return "/bin/bash"
}

// buildShellArgv wraps a shell string command for the user's login shell.
func buildShellArgv(command string) []string {
	return []string{detectUserShell(), "-lc", command}
}

func runCommand(ctx context.Context, workdir string, argv []string, timeoutMs int) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("command required")
	}
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctxTimeout, argv[0], argv[1:]...)
	cmd.Dir = workdir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	outText := buildOutput(stdout.Bytes(), stderr.Bytes())
	if ctxTimeout.Err() == context.DeadlineExceeded {
		return outText + "\n[timeout] command exceeded timeout", fmt.Errorf("command timed out")
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return outText + fmt.Sprintf("\n[exit_code] %d", exitErr.ExitCode()), nil
		}
		return outText, err
	}
	if strings.TrimSpace(outText) == "" {
		return "(no output)", nil
	}
	return outText, nil
}

// runCommandToWriter runs argv in workdir, writing stdout+stderr directly to w
// as each chunk arrives. Mirrors forge's fd redirection for background
// Bash tasks: output is visible in the file before the command completes, so
// TaskOutput can poll real-time progress.
func runCommandToWriter(ctx context.Context, workdir string, argv []string, timeoutMs int, w io.Writer) {
	if len(argv) == 0 {
		return
	}
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctxTimeout, argv[0], argv[1:]...)
	cmd.Dir = workdir
	cmd.Stdout = w
	cmd.Stderr = w

	if err := cmd.Run(); err != nil {
		if ctxTimeout.Err() == context.DeadlineExceeded {
			fmt.Fprintln(w, "\n[timeout] command exceeded timeout") //nolint:errcheck
			return
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(w, "\n[exit_code] %d\n", exitErr.ExitCode()) //nolint:errcheck
		}
	}
}

func buildOutput(stdout, stderr []byte) string {
	data := append([]byte{}, stdout...)
	if len(stderr) > 0 {
		if len(data) > 0 && data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}
		data = append(data, stderr...)
	}
	if len(data) > maxOutputBytes {
		data = append(data[:maxOutputBytes], "\n...[truncated]"...)
	}
	return strings.TrimRight(string(data), "\n")
}
