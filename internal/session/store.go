// Package session reads and writes Claude Code session transcripts on disk.
package session

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrNotFound reports that no session with the given ID exists under the store root.
var ErrNotFound = errors.New("session not found")

// Store reads and writes sessions under a Claude Code config root such as ~/.claude.
type Store struct {
	root string
}

// NewStore returns a Store rooted at a Claude Code config dir.
func NewStore(root string) *Store {
	return &Store{root: root}
}

func (s *Store) projectsDir() string {
	return filepath.Join(s.root, "projects")
}

// locate returns the transcript path for a session ID.
func (s *Store) locate(id string) (string, error) {
	if id == "" || strings.ContainsAny(id, `/\*?[`) {
		return "", fmt.Errorf("%w: invalid id %q", ErrNotFound, id)
	}
	matches, err := filepath.Glob(filepath.Join(s.projectsDir(), "*", id+".jsonl"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("%w: %s under %s", ErrNotFound, id, s.projectsDir())
	}
	return matches[0], nil
}

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9]`)

// projectDirName encodes a cwd the way Claude Code names project folders.
func projectDirName(cwd string) string {
	return nonAlnum.ReplaceAllString(cwd, "-")
}
