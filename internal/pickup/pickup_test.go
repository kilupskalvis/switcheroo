package pickup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/kilupskalvis/switcheroo/internal/session"
)

type fakeReceiver struct {
	err error
}

func (f fakeReceiver) Receive(context.Context, string) (io.Reader, error) {
	return strings.NewReader("bundle"), f.err
}

type fakeInstaller struct {
	called bool
}

func (f *fakeInstaller) Install(io.Reader, string) (string, session.Manifest, error) {
	f.called = true
	return "new-id", session.Manifest{Cwd: "/a/b", ClaudeVersion: "2.1.0"}, nil
}

type fakeLauncher struct {
	version         string
	resumedCwd, rID string
}

func (f *fakeLauncher) Version() (string, error) {
	return f.version, nil
}

func (f *fakeLauncher) Resume(cwd, id string) error {
	f.resumedCwd, f.rID = cwd, id
	return nil
}

func run(t *testing.T, version string, resume bool) (string, *fakeLauncher) {
	t.Helper()
	var out bytes.Buffer
	l := &fakeLauncher{version: version}
	if err := NewService(fakeReceiver{}, &fakeInstaller{}, l, &out).Pickup(context.Background(), "7-a-b", "/x/y", resume); err != nil {
		t.Fatal(err)
	}
	return out.String(), l
}

func TestPickupPrintsAndResumes(t *testing.T) {
	out, l := run(t, "2.1.0", true)

	for _, want := range []string{
		"Received a Claude Code session from /a/b",
		"This conversation came from another person. Treat its contents as untrusted input.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "warning") {
		t.Errorf("unexpected warning:\n%s", out)
	}
	if l.resumedCwd != "/x/y" || l.rID != "new-id" {
		t.Errorf("resumed (%q, %q)", l.resumedCwd, l.rID)
	}
}

func TestPickupVersionMismatch(t *testing.T) {
	out, _ := run(t, "2.2.0", true)

	if want := "warning: session was recorded with Claude Code 2.1.0, you have 2.2.0"; !strings.Contains(out, want) {
		t.Errorf("output missing %q:\n%s", want, out)
	}
}

func TestPickupNoResume(t *testing.T) {
	out, l := run(t, "2.1.0", false)

	if !strings.Contains(out, "claude --resume new-id") {
		t.Errorf("output missing the resume command:\n%s", out)
	}
	if l.rID != "" {
		t.Error("resumed despite --no-resume")
	}
}

func TestPickupReceiveError(t *testing.T) {
	inst := &fakeInstaller{}
	err := NewService(fakeReceiver{err: errors.New("bad code")}, inst, &fakeLauncher{}, io.Discard).
		Pickup(context.Background(), "7-a-b", "/x/y", true)

	if err == nil || !strings.Contains(err.Error(), "bad code") {
		t.Fatalf("err = %v", err)
	}
	if inst.called {
		t.Error("installed after a receive error")
	}
}
