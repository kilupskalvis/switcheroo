package session

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"
)

// AwaitCall waits until the transcript records a still-running tool call whose command contains command.
// Claude Code writes a reply shortly after its tool calls start, so this proves the reply is on disk.
func (s *Store) AwaitCall(ctx context.Context, id, command string) error {
	path, err := s.locate(id)
	if err != nil {
		return err
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		found, err := hasPendingCall(path, command)
		if err != nil || found {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

// hasPendingCall reports whether the transcript holds a matching tool call that has no result yet.
func hasPendingCall(path, command string) (bool, error) {
	f, err := os.Open(path) //nolint:gosec // path comes from the store's own glob
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()

	pending := map[string]bool{}
	r := bufio.NewReader(f)
	for {
		line, readErr := r.ReadBytes('\n')
		var e struct {
			Message struct {
				Content []struct {
					Type      string `json:"type"`
					ID        string `json:"id"`
					ToolUseID string `json:"tool_use_id"`
					Input     struct {
						Command string `json:"command"`
					} `json:"input"`
				} `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &e) == nil {
			for _, c := range e.Message.Content {
				switch {
				case c.Type == "tool_use" && strings.Contains(c.Input.Command, command):
					pending[c.ID] = true
				case c.Type == "tool_result":
					delete(pending, c.ToolUseID)
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			return len(pending) > 0, nil
		}
		if readErr != nil {
			return false, readErr
		}
	}
}
