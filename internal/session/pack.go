package session

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Pack writes the session with the given ID and the sender's handoff note to w as a gzipped tar bundle, unchanged.
func (s *Store) Pack(id, note string, w io.Writer) error {
	file, err := s.locate(id)
	if err != nil {
		return err
	}
	cwd, version, err := scanTranscript(file)
	if err != nil {
		return err
	}
	dir := strings.TrimSuffix(file, ".jsonl")
	manifest, err := json.Marshal(Manifest{
		Format:        FormatVersion,
		SessionID:     id,
		Cwd:           cwd,
		SessionDir:    dir,
		ClaudeVersion: version,
		CreatedAt:     time.Now().UTC(),
		Note:          note,
	})
	if err != nil {
		return err
	}

	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	if err := writeEntry(tw, "manifest.json", int64(len(manifest)), bytes.NewReader(manifest)); err != nil {
		return err
	}
	if err := addFile(tw, "session.jsonl", file); err != nil {
		return err
	}
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		return addFile(tw, "session/"+filepath.ToSlash(rel), path)
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// addFile copies a file into the tar, up to its size at open time since live sessions keep growing.
func addFile(tw *tar.Writer, name, path string) error {
	f, err := os.Open(path) //nolint:gosec // path comes from the store's own session folder
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	return writeEntry(tw, name, info.Size(), f)
}

func writeEntry(tw *tar.Writer, name string, size int64, r io.Reader) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: size, Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	_, err := io.CopyN(tw, r, size)
	return err
}
