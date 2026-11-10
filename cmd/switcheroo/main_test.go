package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
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
	if !strings.Contains(stderr.String(), "note") {
		t.Errorf("error does not ask for the note:\n%s", stderr.String())
	}
}

func TestSendRequiresHandoffMessageInClaudeCode(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("CLAUDECODE", "1")
	proj := filepath.Join(root, "projects", "-a-b")
	if err := os.MkdirAll(proj, 0o700); err != nil {
		t.Fatal(err)
	}
	call := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"switcheroo send --session s1"}}]}}`
	if err := os.WriteFile(filepath.Join(proj, "s1.jsonl"), []byte(call+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	if code := run([]string{"send", "--session", "s1"}, strings.NewReader("Short note."), io.Discard, &stderr); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "handoff message") {
		t.Errorf("error does not ask for the handoff message:\n%s", stderr.String())
	}
}
