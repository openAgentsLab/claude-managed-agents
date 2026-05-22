package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	defaultGitTimeout = 30 * time.Second
	maxGitOutputBytes = 256 * 1024
)

// runGit executes a git command and returns its combined output.
// A non-zero exit code is returned as an error whose message is the command output.
func runGit(ctx context.Context, workdir string, args ...string) (string, error) {
	out, code, err := runGitRaw(ctx, workdir, args...)
	if err != nil {
		return "", err
	}
	if code != 0 {
		if out != "" {
			return "", fmt.Errorf("%s", out)
		}
		return "", fmt.Errorf("git %s: exit code %d", args[0], code)
	}
	if strings.TrimSpace(out) == "" {
		return "(nothing to show)", nil
	}
	return out, nil
}

// runGitPermissive is like runGit but treats exit code 1 as success.
// Use for commands where exit 1 is a normal non-error state (e.g. git diff
// exits 1 when there are differences; git grep exits 1 when no match).
func runGitPermissive(ctx context.Context, workdir string, args ...string) (string, error) {
	out, code, err := runGitRaw(ctx, workdir, args...)
	if err != nil {
		return "", err
	}
	if code > 1 {
		return "", fmt.Errorf("git %s: exit code %d: %s", args[0], code, out)
	}
	if strings.TrimSpace(out) == "" {
		return "(nothing to show)", nil
	}
	return out, nil
}

func runGitRaw(ctx context.Context, workdir string, args ...string) (output string, exitCode int, err error) {
	ctx, cancel := context.WithTimeout(ctx, defaultGitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workdir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	out := stdout.String()
	if stderr.Len() > 0 {
		if out != "" && out[len(out)-1] != '\n' {
			out += "\n"
		}
		out += stderr.String()
	}

	if ctx.Err() == context.DeadlineExceeded {
		return out, -1, fmt.Errorf("git command timed out")
	}
	if len(out) > maxGitOutputBytes {
		out = out[:maxGitOutputBytes] + "\n...[truncated]"
	}
	out = strings.TrimRight(out, "\n")

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			return out, exitErr.ExitCode(), nil
		}
		return "", -1, runErr
	}
	return out, 0, nil
}
