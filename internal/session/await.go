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
		call, err := pendingCall(path, command)
		if err != nil || call.toolUseID != "" {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

// toolCall identifies a tool call and the transcript entry that holds it.
type toolCall struct {
	toolUseID, entryUUID string
}

// pendingCall returns the last tool call whose command contains command and that has no result yet; zero if none.
func pendingCall(path, command string) (toolCall, error) {
	f, err := os.Open(path) //nolint:gosec // path comes from the store or our own install
	if err != nil {
		return toolCall{}, err
	}
	defer func() { _ = f.Close() }()

	var last toolCall
	pending := map[string]bool{}
	r := bufio.NewReader(f)
	for {
		line, readErr := r.ReadBytes('\n')
		var e struct {
			UUID    string `json:"uuid"`
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
					last = toolCall{toolUseID: c.ID, entryUUID: e.UUID}
				case c.Type == "tool_result":
					delete(pending, c.ToolUseID)
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			if !pending[last.toolUseID] {
				return toolCall{}, nil
			}
			return last, nil
		}
		if readErr != nil {
			return toolCall{}, readErr
		}
	}
}
