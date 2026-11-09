package session

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureID = "11111111-2222-3333-4444-555555555555"

var fixtureLines = []string{
	`{"type":"user","cwd":"/a/b","sessionId":"` + fixtureID + `","version":"2.0.0"}`,
	`{"type":"assistant","cwd":"/a/b/sub","sessionId":"` + fixtureID + `"}`,
	`{"type":"user","cwd":"/a/b","sessionId":"` + fixtureID + `","version":"2.1.0"}`,
}

// writeFixture creates a session under root and returns the project dir.
func writeFixture(t *testing.T, root string, lines []string, sidecar map[string]string) string {
	t.Helper()
	proj := filepath.Join(root, "projects", "-a-b")
	if err := os.MkdirAll(proj, 0o700); err != nil {
		t.Fatal(err)
	}
	data := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(proj, fixtureID+".jsonl"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, content := range sidecar {
		p := filepath.Join(proj, fixtureID, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return proj
}

// untar reads a gzipped tar into ordered names and contents.
func untar(t *testing.T, r io.Reader) ([]string, map[string][]byte) {
	t.Helper()
	gz, err := gzip.NewReader(r)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var names []string
	files := map[string][]byte{}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return names, files
		}
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
		files[h.Name] = b
	}
}

func TestPackRoundTrip(t *testing.T) {
	root := t.TempDir()
	proj := writeFixture(t, root, fixtureLines, map[string]string{"tool-results/x.txt": "big output"})

	var buf bytes.Buffer
	if err := NewStore(root).Pack(fixtureID, &buf); err != nil {
		t.Fatal(err)
	}
	names, files := untar(t, &buf)

	if names[0] != "manifest.json" {
		t.Fatalf("first entry = %q, want manifest.json", names[0])
	}
	var m Manifest
	if err := json.Unmarshal(files["manifest.json"], &m); err != nil {
		t.Fatal(err)
	}
	want := Manifest{Format: FormatVersion, SessionID: fixtureID, Cwd: "/a/b", SessionDir: filepath.Join(proj, fixtureID), ClaudeVersion: "2.1.0"}
	m.CreatedAt = want.CreatedAt
	if m != want {
		t.Errorf("manifest = %+v, want %+v", m, want)
	}
	orig, _ := os.ReadFile(filepath.Join(proj, fixtureID+".jsonl"))
	if !bytes.Equal(files["session.jsonl"], orig) {
		t.Error("session.jsonl differs from the original")
	}
	if string(files["session/tool-results/x.txt"]) != "big output" {
		t.Errorf("sidecar file = %q", files["session/tool-results/x.txt"])
	}
}

func TestPackWithoutSidecar(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, fixtureLines, nil)

	var buf bytes.Buffer
	if err := NewStore(root).Pack(fixtureID, &buf); err != nil {
		t.Fatal(err)
	}
	names, _ := untar(t, &buf)
	for _, n := range names {
		if strings.HasPrefix(n, "session/") {
			t.Errorf("unexpected sidecar entry %q", n)
		}
	}
}

func TestPackUnknownID(t *testing.T) {
	root := t.TempDir()
	err := NewStore(root).Pack("nope", io.Discard)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), root) {
		t.Errorf("error %q does not name the root", err)
	}
}

func TestProjectDirName(t *testing.T) {
	for in, want := range map[string]string{
		"/home/kalvis/repos/personal": "-home-kalvis-repos-personal",
		"/a/b.c_d":                    "-a-b-c-d",
	} {
		if got := projectDirName(in); got != want {
			t.Errorf("projectDirName(%q) = %q, want %q", in, got, want)
		}
	}
}
