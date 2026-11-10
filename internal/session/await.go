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

// ErrNoHandoffMessage means the send call was not directly preceded by Claude's handoff message.
var ErrNoHandoffMessage = errors.New("write the handoff message for the next person as your reply, then run switcheroo send right after it")

// AwaitCall waits until the transcript records a still-running tool call whose command contains command.
// Claude Code writes a reply shortly after its tool calls start, so this proves the reply is on disk.
// It returns ErrNoHandoffMessage when Claude's text does not come directly before the call.
func (s *Store) AwaitCall(ctx context.Context, id, command string) error {
	path, err := s.locate(id)
	if err != nil {
		return err
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		call, err := pendingCall(path, command)
		if err != nil {
			return err
		}
		if call.toolUseID != "" {
			if !call.afterText {
				return ErrNoHandoffMessage
			}
			return nil
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
	// afterText is whether Claude's text came directly before the call, in the same turn.
	afterText bool
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
	afterText := false
	r := bufio.NewReader(f)
	for {
		line, readErr := r.ReadBytes('\n')
		var e struct {
			Type    string `json:"type"`
			UUID    string `json:"uuid"`
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		var blocks []struct {
			Type      string `json:"type"`
			ID        string `json:"id"`
			ToolUseID string `json:"tool_use_id"`
			Input     struct {
				Command string `json:"command"`
			} `json:"input"`
		}
		// Only messages count: logged API requests (api-request-blob) repeat earlier ones, answered calls included.
		if json.Unmarshal(line, &e) == nil && (e.Type == "user" || e.Type == "assistant") {
			if json.Unmarshal(e.Message.Content, &blocks) != nil && e.Type == "user" {
				afterText = false // a prompt typed as plain text starts a new turn
			}
			for _, c := range blocks {
				switch c.Type {
				case "text":
					afterText = e.Type == "assistant"
				case "tool_use":
					if strings.Contains(c.Input.Command, command) {
						pending[c.ID] = true
						last = toolCall{toolUseID: c.ID, entryUUID: e.UUID, afterText: afterText}
					}
					afterText = false
				case "tool_result":
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
