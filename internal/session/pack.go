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

// Pack writes the session with the given ID and the sender's note to w as a gzipped tar bundle,
// with the text Claude Code copied in from the sender's CLAUDE.md, memory and email removed.
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

	// Bundle names in order: the transcript, then its sidecar files.
	names, paths := []string{"session.jsonl"}, []string{file}
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		names, paths = append(names, "session/"+filepath.ToSlash(rel)), append(paths, path)
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	private := map[string]bool{}
	for _, p := range paths {
		if strings.HasSuffix(p, ".jsonl") {
			if err := privateText(p, private); err != nil {
				return err
			}
		}
	}

	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	if err := writeEntry(tw, "manifest.json", int64(len(manifest)), bytes.NewReader(manifest)); err != nil {
		return err
	}
	for i, p := range paths {
		if err := addFile(tw, names[i], p, private); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// addFile copies a file into the tar with the private text removed, up to its size at open
// time since live sessions keep growing.
func addFile(tw *tar.Writer, name, path string, private map[string]bool) error {
	f, err := os.Open(path) //nolint:gosec // path comes from the store's own session folder
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	b, err := io.ReadAll(io.LimitReader(f, info.Size()))
	if err != nil {
		return err
	}
	b = redact(b, private)
	return writeEntry(tw, name, int64(len(b)), bytes.NewReader(b))
}

func writeEntry(tw *tar.Writer, name string, size int64, r io.Reader) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: size, Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	_, err := io.CopyN(tw, r, size)
	return err
}
