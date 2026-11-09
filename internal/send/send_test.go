package send

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

type fakePacker struct {
	err error
}

func (f fakePacker) Pack(id, note string, w io.Writer) error {
	if f.err != nil {
		return f.err
	}
	_, err := io.WriteString(w, "bundle-"+id+"-"+note)
	return err
}

type fakeOfferer struct {
	done chan error
	got  []byte
}

func (f *fakeOfferer) Offer(_ context.Context, r io.ReadSeeker) (string, <-chan error, error) {
	f.got, _ = io.ReadAll(r)
	return "7-a-b", f.done, nil
}

func emptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > 0 {
		t.Errorf("dir not empty: %v", entries)
	}
}

func TestSnapshotRemovesOnPackError(t *testing.T) {
	dir := t.TempDir()
	if _, err := NewService(fakePacker{err: errors.New("boom")}, nil, dir).Snapshot("abc", ""); err == nil {
		t.Fatal("expected the pack error")
	}
	emptyDir(t, dir)
}

func TestServeWritesCodeAndRemoves(t *testing.T) {
	dir := t.TempDir()
	off := &fakeOfferer{done: make(chan error, 1)}
	off.done <- nil
	s := NewService(fakePacker{}, off, dir)
	path, err := s.Snapshot("abc", "note")
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("snapshot %s: %v, %v", path, info, err)
	}

	var out bytes.Buffer
	if err := s.Serve(context.Background(), path, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7-a-b\n" {
		t.Errorf("code output = %q", out.String())
	}
	if string(off.got) != "bundle-abc-note" {
		t.Errorf("offered %q", off.got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("snapshot still exists: %v", err)
	}
	hs, err := s.Handoffs()
	if err != nil || len(hs) != 1 || hs[0].Code != "7-a-b" || hs[0].Session != "abc" || hs[0].State != PickedUp {
		t.Errorf("handoffs = %+v, %v", hs, err)
	}
}

func TestServeTimeout(t *testing.T) {
	dir := t.TempDir()
	s := NewService(fakePacker{}, &fakeOfferer{done: make(chan error)}, dir)
	path, err := s.Snapshot("abc", "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := s.Serve(ctx, path, io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("snapshot still exists: %v", err)
	}
	if hs, _ := s.Handoffs(); len(hs) != 1 || hs[0].State != Cancelled {
		t.Errorf("handoffs = %+v, want one cancelled", hs)
	}
}

// waitingHandoff records a waiting handoff for the given sender process.
func waitingHandoff(t *testing.T, dir, code string, pid int) {
	t.Helper()
	b, _ := json.Marshal(Handoff{Code: code, Session: "abc", PID: pid, Started: time.Now(), State: Waiting})
	if err := os.WriteFile(filepath.Join(dir, "abc-1.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCancelStopsWaitingSender(t *testing.T) {
	dir := t.TempDir()
	sender := exec.CommandContext(context.Background(), "sleep", "30")
	if err := sender.Start(); err != nil {
		t.Fatal(err)
	}
	waitingHandoff(t, dir, "7-a-b", sender.Process.Pid)
	s := NewService(nil, nil, dir)

	if err := s.Cancel("1-x-y"); err == nil {
		t.Error("cancelled an unknown code")
	}
	if err := s.Cancel("7-a-b"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- sender.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = sender.Process.Kill()
		t.Fatal("sender still running after cancel")
	}
}

func TestHandoffsMarksGoneSenderFailed(t *testing.T) {
	dir := t.TempDir()
	gone := exec.CommandContext(context.Background(), "true")
	if err := gone.Run(); err != nil {
		t.Fatal(err)
	}
	waitingHandoff(t, dir, "7-a-b", gone.Process.Pid)

	if hs, _ := NewService(nil, nil, dir).Handoffs(); len(hs) != 1 || hs[0].State != Failed {
		t.Errorf("handoffs = %+v, want one failed", hs)
	}
}
