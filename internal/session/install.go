package session

import (
	"archive/tar"
	"compress/gzip"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MaxBundleBytes caps the unpacked size of a received bundle.
const MaxBundleBytes = 4 << 30

var maxBytes int64 = MaxBundleBytes

// Install unpacks a received bundle as a new session for cwd and returns its new ID and manifest.
func (s *Store) Install(r io.Reader, cwd string) (id string, m Manifest, err error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", m, fmt.Errorf("bundle: %w", err)
	}
	limited := &io.LimitedReader{R: gz, N: maxBytes + 1}
	tr := tar.NewReader(limited)
	var transcript, sidecar string
	defer func() {
		if limited.N <= 0 {
			id, err = "", fmt.Errorf("bundle: larger than %d bytes", maxBytes)
		}
		if err != nil && transcript != "" {
			_ = os.Remove(transcript)
			_ = os.RemoveAll(sidecar)
		}
	}()

	if m, err = readManifest(tr); err != nil {
		return "", m, err
	}
	if id, err = newUUID(); err != nil {
		return "", m, err
	}
	project := filepath.Join(s.projectsDir(), projectDirName(cwd))
	transcript = filepath.Join(project, id+".jsonl")
	sidecar = filepath.Join(project, id)

	rw := newRewriter(m, id, cwd, sidecar)
	for {
		h, nextErr := tr.Next()
		if errors.Is(nextErr, io.EOF) {
			return id, m, nil
		}
		if nextErr != nil {
			return "", m, fmt.Errorf("bundle: %w", nextErr)
		}
		dest, rewrite, err := target(h, transcript, sidecar)
		if err != nil {
			return "", m, err
		}
		if err := writeFile(dest, tr, rewrite, rw); err != nil {
			return "", m, err
		}
	}
}

func readManifest(tr *tar.Reader) (Manifest, error) {
	var m Manifest
	h, err := tr.Next()
	if err != nil {
		return m, fmt.Errorf("bundle: %w", err)
	}
	if h.Name != "manifest.json" {
		return m, errors.New("bundle: manifest.json must come first")
	}
	if err := json.NewDecoder(tr).Decode(&m); err != nil {
		return m, fmt.Errorf("bundle manifest: %w", err)
	}
	if m.Format != FormatVersion {
		return m, fmt.Errorf("bundle format %d is not supported, update switcheroo", m.Format)
	}
	if m.SessionID == "" || m.SessionDir == "" {
		return m, errors.New("bundle manifest: missing session ID or folder")
	}
	return m, nil
}

// target maps a bundle entry to its install path; only regular files inside the session are allowed.
func target(h *tar.Header, transcript, sidecar string) (dest string, rewrite bool, err error) {
	if h.Typeflag != tar.TypeReg {
		return "", false, fmt.Errorf("bundle: %q is not a regular file", h.Name)
	}
	if h.Name == "session.jsonl" {
		return transcript, true, nil
	}
	rel, ok := strings.CutPrefix(h.Name, "session/")
	if !ok || !filepath.IsLocal(rel) {
		return "", false, fmt.Errorf("bundle: unexpected entry %q", h.Name)
	}
	isAgent := strings.HasPrefix(rel, "subagents/") && strings.HasSuffix(rel, ".jsonl")
	return filepath.Join(sidecar, filepath.FromSlash(rel)), isAgent, nil
}

func writeFile(dest string, r io.Reader, rewrite bool, rw rewriter) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // dest is checked by target
	if err != nil {
		return err
	}
	if rewrite {
		err = rw.copy(f, r)
	} else {
		_, err = io.Copy(f, r)
	}
	return errors.Join(err, f.Close())
}

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
