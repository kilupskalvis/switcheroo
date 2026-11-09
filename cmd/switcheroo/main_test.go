package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

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
