package send

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"time"
)

// Handoff states.
const (
	Waiting   = "waiting"
	PickedUp  = "picked up"
	Expired   = "expired"
	Cancelled = "cancelled"
	Failed    = "failed"
)

// Handoff is one sent session as recorded next to its snapshot.
type Handoff struct {
	Code    string    `json:"code"`
	Session string    `json:"session"`
	PID     int       `json:"pid"`
	Started time.Time `json:"started"`
	State   string    `json:"state"`
	Ended   time.Time `json:"ended,omitzero"`
}

// Handoffs returns the recorded handoffs, newest first; a waiting one whose sender is gone counts as failed.
func (s *Service) Handoffs() ([]Handoff, error) {
	paths, err := filepath.Glob(filepath.Join(s.dir, "*.json"))
	if err != nil {
		return nil, err
	}
	hs := make([]Handoff, 0, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p) //nolint:gosec // p comes from our own state dir
		if err != nil {
			return nil, err
		}
		var h Handoff
		if json.Unmarshal(b, &h) != nil {
			continue
		}
		if h.State == Waiting && !alive(h.PID) {
			h.State = Failed
		}
		hs = append(hs, h)
	}
	slices.SortFunc(hs, func(a, b Handoff) int { return b.Started.Compare(a.Started) })
	return hs, nil
}

// Cancel stops the sender still waiting on code; it removes the snapshot and records the handoff as cancelled.
func (s *Service) Cancel(code string) error {
	hs, err := s.Handoffs()
	if err != nil {
		return err
	}
	for _, h := range hs {
		if h.Code == code && h.State == Waiting {
			return syscall.Kill(h.PID, syscall.SIGTERM)
		}
	}
	return fmt.Errorf("no handoff is waiting on code %s", code)
}

func outcome(err error) string {
	switch {
	case err == nil:
		return PickedUp
	case errors.Is(err, context.Canceled):
		return Cancelled
	case errors.Is(err, context.DeadlineExceeded):
		return Expired
	default:
		return Failed
	}
}

func writeState(path string, h Handoff) error {
	b, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// alive reports whether a process with pid exists.
func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
