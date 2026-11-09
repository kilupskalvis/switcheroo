package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// spawnServe starts a detached `switcheroo serve` for bundle and returns the code it prints.
func spawnServe(bundle string, flags []string, logPath string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec // logPath sits next to our own snapshot
	if err != nil {
		return "", err
	}
	defer func() { _ = logFile.Close() }()

	args := append([]string{"serve", "--bundle", bundle}, flags...)
	cmd := exec.CommandContext(context.Background(), exe, args...) //nolint:gosec // re-executes this binary
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}

	codes := make(chan string, 1)
	go func() {
		s := bufio.NewScanner(stdout)
		if s.Scan() {
			codes <- s.Text()
		}
		close(codes)
	}()
	select {
	case code, ok := <-codes:
		if !ok {
			return "", fmt.Errorf("sender exited without a code, see %s", logPath)
		}
		return code, cmd.Process.Release()
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		return "", errors.New("timed out waiting for the relay to issue a code")
	}
}
