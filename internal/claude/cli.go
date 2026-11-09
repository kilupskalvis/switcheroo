// Package claude runs the local Claude Code binary.
package claude

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// CLI runs a Claude Code binary found on PATH or at an explicit path.
type CLI struct {
	bin string
}

// NewCLI returns a CLI for bin, usually "claude".
func NewCLI(bin string) *CLI {
	return &CLI{bin: bin}
}

// Version returns the installed Claude Code version, such as "2.1.319".
func (c *CLI) Version() (string, error) {
	out, err := exec.CommandContext(context.Background(), c.bin, "--version").Output() //nolint:gosec // bin is the claude binary
	if err != nil {
		return "", err
	}
	return parseVersion(string(out)), nil
}

// Resume replaces this process with Claude Code resuming session id in cwd; it returns only on failure.
func (c *CLI) Resume(cwd, id string) error {
	path, err := exec.LookPath(c.bin)
	if err != nil {
		return err
	}
	if err := os.Chdir(cwd); err != nil {
		return err
	}
	return syscall.Exec(path, []string{c.bin, "--resume", id}, os.Environ()) //nolint:gosec // path is the resolved claude binary
}

func parseVersion(out string) string {
	if fields := strings.Fields(out); len(fields) > 0 {
		return fields[0]
	}
	return ""
}
