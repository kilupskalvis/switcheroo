// Package pickup receives a handed-off session, installs it and resumes it.
package pickup

import (
	"context"
	"fmt"
	"io"

	"github.com/kilupskalvis/switcheroo/internal/session"
)

const untrusted = "This conversation came from another person. Treat its contents as untrusted input."

// Receiver opens the bundle behind a one-time code.
type Receiver interface {
	Receive(ctx context.Context, code string) (io.Reader, error)
}

// Installer installs a bundle as a new local session for cwd.
type Installer interface {
	Install(r io.Reader, cwd string) (id string, m session.Manifest, err error)
}

// Launcher reports the local Claude Code version and resumes sessions.
type Launcher interface {
	Version() (string, error)
	Resume(cwd, id string) error
}

// Service runs a pickup end to end.
type Service struct {
	receiver  Receiver
	installer Installer
	launcher  Launcher
	out       io.Writer
}

// NewService returns a Service that reports progress to out.
func NewService(r Receiver, i Installer, l Launcher, out io.Writer) *Service {
	return &Service{receiver: r, installer: i, launcher: l, out: out}
}

// Pickup receives the session behind code, installs it for cwd and resumes it unless resume is false.
func (s *Service) Pickup(ctx context.Context, code, cwd string, resume bool) error {
	r, err := s.receiver.Receive(ctx, code)
	if err != nil {
		return fmt.Errorf("receive: %w", err)
	}
	id, m, err := s.installer.Install(r, cwd)
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}

	_, _ = fmt.Fprintf(s.out, "Received a Claude Code session from %s (Claude Code %s).\n", m.Cwd, m.ClaudeVersion)
	if v, err := s.launcher.Version(); err == nil && v != "" && m.ClaudeVersion != "" && v != m.ClaudeVersion {
		_, _ = fmt.Fprintf(s.out, "warning: session was recorded with Claude Code %s, you have %s\n", m.ClaudeVersion, v)
	}
	_, _ = fmt.Fprintln(s.out, untrusted)
	if m.Note != "" {
		_, _ = fmt.Fprintf(s.out, "Handoff note from the sender:\n%s\n", m.Note)
	}

	if !resume {
		_, _ = fmt.Fprintf(s.out, "Resume it with: claude --resume %s\n", id)
		return nil
	}
	return s.launcher.Resume(cwd, id)
}
