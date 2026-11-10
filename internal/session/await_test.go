package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	sendCall   = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Handoff: ..."},{"type":"tool_use","id":"t2","name":"Bash","input":{"command":"switcheroo send --session x"}}]}}`
	oldCall    = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"switcheroo send --session x"}}]}}`
	oldResult  = `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"Pickup code: 1-a-b"}]}}`
	userPrompt = `{"type":"user","message":{"role":"user","content":"/switcheroo"}}`
)

func TestAwaitCallReturnsOnceCallIsWritten(t *testing.T) {
	root := t.TempDir()
	proj := writeFixture(t, root, []string{userPrompt}, nil)
	go func() {
		time.Sleep(200 * time.Millisecond)
		f, err := os.OpenFile(filepath.Join(proj, fixtureID+".jsonl"), os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			return
		}
		_, _ = f.WriteString(sendCall + "\n")
		_ = f.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := NewStore(root).AwaitCall(ctx, fixtureID, "switcheroo send"); err != nil {
		t.Fatalf("AwaitCall = %v, want nil", err)
	}
}

func TestAwaitCallRequiresHandoffMessage(t *testing.T) {
	const (
		text     = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Handoff: ..."}]}}`
		thinking = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"..."}]}}`
		bareCall = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t2","name":"Bash","input":{"command":"switcheroo send --session x"}}]}}`
		lsCall   = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t9","name":"Bash","input":{"command":"ls"}}]}}`
		lsResult = `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t9","content":"a"}]}}`
	)
	for _, tc := range []struct {
		name  string
		lines []string
		want  error
	}{
		{"message in the same entry", []string{userPrompt, sendCall}, nil},
		{"message then thinking", []string{userPrompt, text, thinking, bareCall}, nil},
		{"no message", []string{userPrompt, bareCall}, ErrNoHandoffMessage},
		{"message from the previous turn", []string{text, userPrompt, bareCall}, ErrNoHandoffMessage},
		{"message before another tool call", []string{userPrompt, text, lsCall, lsResult, bareCall}, ErrNoHandoffMessage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFixture(t, root, tc.lines, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := NewStore(root).AwaitCall(ctx, fixtureID, "switcheroo send"); !errors.Is(err, tc.want) {
				t.Errorf("AwaitCall = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestAwaitCallIgnoresAnsweredCall(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, []string{userPrompt, oldCall, oldResult}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := NewStore(root).AwaitCall(ctx, fixtureID, "switcheroo send"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("AwaitCall = %v, want deadline exceeded", err)
	}
}
