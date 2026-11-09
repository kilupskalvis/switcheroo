package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

// FormatVersion is the bundle layout version written into every manifest.
const FormatVersion = 1

// Manifest describes a bundled session; it is the first entry of every bundle.
type Manifest struct {
	Format        int       `json:"format"`
	SessionID     string    `json:"sessionId"`
	Cwd           string    `json:"cwd"`
	SessionDir    string    `json:"sessionDir"`
	ClaudeVersion string    `json:"claudeVersion"`
	CreatedAt     time.Time `json:"createdAt"`
	Note          string    `json:"note,omitempty"`
}

// scanTranscript reads the first cwd and the last Claude Code version from a transcript.
func scanTranscript(path string) (cwd, version string, err error) {
	f, err := os.Open(path) //nolint:gosec // path comes from the store's own glob
	if err != nil {
		return "", "", err
	}
	defer func() { _ = f.Close() }()

	r := bufio.NewReader(f)
	for {
		line, readErr := r.ReadBytes('\n')
		var e struct{ Cwd, Version string }
		if json.Unmarshal(line, &e) == nil {
			if cwd == "" {
				cwd = e.Cwd
			}
			if e.Version != "" {
				version = e.Version
			}
		}
		if errors.Is(readErr, io.EOF) {
			return cwd, version, nil
		}
		if readErr != nil {
			return "", "", readErr
		}
	}
}
