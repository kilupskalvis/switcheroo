package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
)

// redacted stands in for text Claude Code copied in from the sender's own setup.
var redacted = []byte("(removed by switcheroo)")

// minPrivate keeps a near-empty instruction file from blanking matching text all over the transcript.
const minPrivate = 16

// privateText adds to set the sender's instruction files (CLAUDE.md, memory) and email context
// recorded in the transcript at path, each in the escaped form it takes inside a JSON string.
func privateText(path string, set map[string]bool) error {
	f, err := os.Open(path) //nolint:gosec // path comes from the store's own session folder
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	r := bufio.NewReader(f)
	for {
		line, readErr := r.ReadBytes('\n')
		var e struct {
			Attachment struct {
				Type  string `json:"type"`
				Files []struct {
					Content string `json:"content"`
				} `json:"files"`
				Context struct {
					UserEmail string `json:"userEmail"`
				} `json:"context"`
			} `json:"attachment"`
		}
		if bytes.Contains(line, []byte(`"attachment"`)) && json.Unmarshal(line, &e) == nil {
			var texts []string
			switch e.Attachment.Type {
			case "instructions":
				for _, file := range e.Attachment.Files {
					texts = append(texts, file.Content)
				}
			case "session_context":
				texts = append(texts, e.Attachment.Context.UserEmail)
			}
			for _, t := range texts {
				if len(t) >= minPrivate {
					set[string(jsonEscape(t))] = true
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

// jsonEscape returns s as it appears inside a JSON string written by Claude Code.
// shortcut: Go also escapes U+2028 and U+2029, which JavaScript leaves as is, so text holding them is not matched.
func jsonEscape(s string) []byte {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	_ = e.Encode(s)
	return bytes.TrimSuffix(bytes.TrimPrefix(bytes.TrimSpace(b.Bytes()), []byte(`"`)), []byte(`"`))
}

// redact replaces every private text in b, longest first so one never leaves part of another behind.
func redact(b []byte, set map[string]bool) []byte {
	private := make([]string, 0, len(set))
	for s := range set {
		private = append(private, s)
	}
	sort.Slice(private, func(i, j int) bool { return len(private[i]) > len(private[j]) })
	for _, s := range private {
		b = bytes.ReplaceAll(b, []byte(s), redacted)
	}
	return b
}
