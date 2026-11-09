package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVersion(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho '9.9.9 (Claude Code)'\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := NewCLI(bin).Version()
	if err != nil {
		t.Fatal(err)
	}
	if got != "9.9.9" {
		t.Errorf("Version() = %q, want 9.9.9", got)
	}
}
