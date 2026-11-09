package main

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeRoot(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/custom")
	if got, _ := claudeRoot(); got != "/custom" {
		t.Errorf("claudeRoot() = %q, want /custom", got)
	}

	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("HOME", "/home/u")
	if got, _ := claudeRoot(); got != filepath.Join("/home/u", ".claude") {
		t.Errorf("claudeRoot() = %q, want /home/u/.claude", got)
	}
}

func TestUsageErrors(t *testing.T) {
	for name, args := range map[string][]string{
		"no command":      nil,
		"unknown command": {"nope"},
		"send no session": {"send"},
		"pickup no code":  {"pickup"},
	} {
		t.Run(name, func(t *testing.T) {
			var stderr bytes.Buffer
			if code := run(args, strings.NewReader(""), io.Discard, &stderr); code != 2 {
				t.Errorf("exit = %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), "usage:") {
				t.Errorf("stderr lacks usage:\n%s", stderr.String())
			}
		})
	}
}

func TestSendUnknownSession(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("CLAUDECODE", "")

	var stderr bytes.Buffer
	if code := run([]string{"send", "--session", "missing"}, strings.NewReader(""), io.Discard, &stderr); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), root) {
		t.Errorf("error does not name the searched root:\n%s", stderr.String())
	}
}

func TestSendRequiresNoteInClaudeCode(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("CLAUDECODE", "1")

	var stderr bytes.Buffer
	if code := run([]string{"send", "--session", "missing"}, strings.NewReader(" \n"), io.Discard, &stderr); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "handoff note") {
		t.Errorf("error does not ask for the handoff note:\n%s", stderr.String())
	}
}
