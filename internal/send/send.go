// Package send snapshots a session and offers it to one receiver under a one-time code.
package send

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

// Packer writes a session as a bundle.
type Packer interface {
	Pack(id string, w io.Writer) error
}

// Offerer makes a bundle available under a one-time code.
type Offerer interface {
	Offer(ctx context.Context, r io.ReadSeeker) (code string, done <-chan error, err error)
}

// Service snapshots sessions into dir and serves each snapshot once.
type Service struct {
	packer  Packer
	offerer Offerer
	dir     string
}

// NewService returns a Service that keeps snapshots in dir.
func NewService(p Packer, o Offerer, dir string) *Service {
	return &Service{packer: p, offerer: o, dir: dir}
}

// Snapshot packs session id into a private file and returns its path.
func (s *Service) Snapshot(id string) (path string, err error) {
	f, err := os.CreateTemp(s.dir, id+"-*.tgz")
	if err != nil {
		return "", err
	}
	if err = errors.Join(s.packer.Pack(id, f), f.Close()); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// Serve offers the snapshot at path, writes its code to codeOut and waits for the pickup; the snapshot is always removed.
func (s *Service) Serve(ctx context.Context, path string, codeOut io.Writer) error {
	defer func() { _ = os.Remove(path) }()
	f, err := os.Open(path) //nolint:gosec // path is a snapshot this service wrote
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	code, done, err := s.offerer.Offer(ctx, f)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(codeOut, code); err != nil {
		return err
	}
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
