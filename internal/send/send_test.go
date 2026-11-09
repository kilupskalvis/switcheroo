package send

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
)

type fakePacker struct {
	err error
}

func (f fakePacker) Pack(id string, w io.Writer) error {
	if f.err != nil {
		return f.err
	}
	_, err := io.WriteString(w, "bundle-"+id)
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
	if _, err := NewService(fakePacker{err: errors.New("boom")}, nil, dir).Snapshot("abc"); err == nil {
		t.Fatal("expected the pack error")
	}
	emptyDir(t, dir)
}

func TestServeWritesCodeAndRemoves(t *testing.T) {
	dir := t.TempDir()
	off := &fakeOfferer{done: make(chan error, 1)}
	off.done <- nil
	s := NewService(fakePacker{}, off, dir)
	path, err := s.Snapshot("abc")
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
	if string(off.got) != "bundle-abc" {
		t.Errorf("offered %q", off.got)
	}
	emptyDir(t, dir)
}

func TestServeTimeout(t *testing.T) {
	dir := t.TempDir()
	s := NewService(fakePacker{}, &fakeOfferer{done: make(chan error)}, dir)
	path, err := s.Snapshot("abc")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := s.Serve(ctx, path, io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	emptyDir(t, dir)
}
