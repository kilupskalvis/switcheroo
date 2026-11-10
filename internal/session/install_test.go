package session

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// packFixture packs a fixture session from a fresh root.
func packFixture(t *testing.T, lines []string, sidecar map[string]string) *bytes.Buffer {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, lines, sidecar)
	var buf bytes.Buffer
	if err := NewStore(root).Pack(fixtureID, "", &buf); err != nil {
		t.Fatal(err)
	}
	return &buf
}

// installedLines installs a bundle under a fresh root for cwd /x/y and returns its ID, root and raw lines.
func installedLines(t *testing.T, bundle *bytes.Buffer) (string, string, []string) {
	t.Helper()
	root := t.TempDir()
	id, _, err := NewStore(root).Install(bundle, "/x/y")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "projects", "-x-y", id+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return id, root, strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func decode(t *testing.T, line string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("line %q: %v", line, err)
	}
	return m
}

// regularFiles lists regular files under dir.
func regularFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			files = append(files, p)
		}
		return nil
	})
	return files
}

func TestInstallRewrites(t *testing.T) {
	srcRoot := t.TempDir()
	oldRef := filepath.Join(srcRoot, "projects", "-a-b", fixtureID, "tool-results", "x.txt")
	lines := []string{
		`{"type":"user","cwd":"/a/b","sessionId":"` + fixtureID + `","permissionMode":"plan","message":{"content":"see ` + oldRef + ` and <b>&"}}`,
		`{"type":"permission-mode","permissionMode":"plan","sessionId":"` + fixtureID + `"}`,
		`{"type":"assistant","cwd":"/a/b/sub","sessionId":"` + fixtureID + `"}`,
		`{"type":"assistant","cwd":"/a/bc","session_id":"` + fixtureID + `"}`,
	}
	writeFixture(t, srcRoot, lines, map[string]string{"tool-results/x.txt": "big"})
	var bundle bytes.Buffer
	if err := NewStore(srcRoot).Pack(fixtureID, "", &bundle); err != nil {
		t.Fatal(err)
	}

	id, root, got := installedLines(t, &bundle)

	if id == fixtureID {
		t.Fatal("install reused the old session ID")
	}
	if len(got) != 3 {
		t.Fatalf("got %d lines, want 3 (permission-mode dropped): %q", len(got), got)
	}
	first, sub, sibling := decode(t, got[0]), decode(t, got[1]), decode(t, got[2])
	if first["cwd"] != "/x/y" || sub["cwd"] != "/x/y/sub" || sibling["cwd"] != "/a/bc" {
		t.Errorf("cwds = %v, %v, %v", first["cwd"], sub["cwd"], sibling["cwd"])
	}
	if first["sessionId"] != id || sub["sessionId"] != id || sibling["session_id"] != id {
		t.Error("session IDs not rewritten")
	}
	if _, ok := first["permissionMode"]; ok {
		t.Error("permissionMode key kept")
	}
	newRef := filepath.Join(root, "projects", "-x-y", id, "tool-results", "x.txt")
	if !strings.Contains(got[0], newRef) || strings.Contains(got[0], oldRef) {
		t.Errorf("sidecar path not rewritten: %s", got[0])
	}
	if !strings.Contains(got[0], "<b>&") {
		t.Errorf("text was HTML-escaped: %s", got[0])
	}
	if b, err := os.ReadFile(newRef); err != nil || string(b) != "big" {
		t.Errorf("sidecar file = %q, %v", b, err)
	}
}

func TestInstallSubagents(t *testing.T) {
	agent := `{"type":"user","cwd":"/a/b","sessionId":"` + fixtureID + `"}`
	meta := `{"agentType":"Explore","sessionId":"` + fixtureID + `"}`
	bundle := packFixture(t, fixtureLines, map[string]string{
		"subagents/agent-1.jsonl":     agent,
		"subagents/agent-1.meta.json": meta,
	})

	id, root, _ := installedLines(t, bundle)

	dir := filepath.Join(root, "projects", "-x-y", id, "subagents")
	data, err := os.ReadFile(filepath.Join(dir, "agent-1.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if a := decode(t, string(data)); a["sessionId"] != id || a["cwd"] != "/x/y" {
		t.Errorf("subagent not rewritten: %s", data)
	}
	if m, _ := os.ReadFile(filepath.Join(dir, "agent-1.meta.json")); string(m) != meta {
		t.Errorf("meta changed: %s", m)
	}
}

func TestInstallCompletesPendingSend(t *testing.T) {
	call := `{"type":"assistant","uuid":"a1","cwd":"/a/b","sessionId":"` + fixtureID + `","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"switcheroo send --session x <<'EOF'\nnote\nEOF"}}]}}`
	root := t.TempDir()
	writeFixture(t, root, []string{fixtureLines[0], call}, nil)
	var bundle bytes.Buffer
	if err := NewStore(root).Pack(fixtureID, "Next: add tests.", &bundle); err != nil {
		t.Fatal(err)
	}

	id, _, got := installedLines(t, &bundle)

	if len(got) != 3 {
		t.Fatalf("got %d lines, want the call answered by a third:\n%s", len(got), strings.Join(got, "\n"))
	}
	res := decode(t, got[2])
	for k, want := range map[string]any{"type": "user", "parentUuid": "a1", "sourceToolAssistantUUID": "a1", "sessionId": id, "cwd": "/x/y", "version": "2.0.0"} {
		if res[k] != want {
			t.Errorf("%s = %v, want %v", k, res[k], want)
		}
	}
	if res["uuid"] == "" || res["timestamp"] == "" {
		t.Errorf("missing uuid or timestamp: %s", got[2])
	}
	content := res["message"].(map[string]any)["content"].([]any)[0].(map[string]any)
	if content["type"] != "tool_result" || content["tool_use_id"] != "t1" {
		t.Errorf("not a result for t1: %v", content)
	}
	if text := content["content"]; text != "Handed off with switcheroo." {
		t.Errorf("result = %q, want only the handoff confirmation", text)
	}
}

func TestInstallLeavesAnsweredSend(t *testing.T) {
	call := `{"type":"assistant","uuid":"a1","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"switcheroo send --session x"}}]}}`
	result := `{"type":"user","uuid":"u1","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"Pickup code: 1-a-b"}]}}`
	bundle := packFixture(t, []string{fixtureLines[0], call, result}, nil)

	if _, _, got := installedLines(t, bundle); len(got) != 3 {
		t.Errorf("got %d lines, want the transcript unchanged", len(got))
	}
}

func TestInstallKeepsNonObjectLines(t *testing.T) {
	bundle := packFixture(t, []string{fixtureLines[0], "", `{"type":"us`}, nil)

	_, _, got := installedLines(t, bundle)

	if len(got) != 3 || got[1] != "" || got[2] != `{"type":"us` {
		t.Errorf("lines = %q", got)
	}
}

type entry struct {
	name, body string
	typeflag   byte
}

// handBundle builds a tar.gz from raw entries.
func handBundle(t *testing.T, entries ...entry) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: 0o600, Size: int64(len(e.body)), Typeflag: e.typeflag}
		if e.typeflag == tar.TypeSymlink {
			h.Linkname, h.Size = "/etc/passwd", 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func TestInstallRejectsHostile(t *testing.T) {
	manifest := func(format int) entry {
		b, _ := json.Marshal(Manifest{Format: format, SessionID: fixtureID, Cwd: "/a/b", SessionDir: "/r/projects/-a-b/" + fixtureID})
		return entry{name: "manifest.json", body: string(b), typeflag: tar.TypeReg}
	}
	jsonl := entry{name: "session.jsonl", body: fixtureLines[0] + "\n", typeflag: tar.TypeReg}
	cases := map[string][]entry{
		"parent escape":   {manifest(1), jsonl, {name: "session/../../evil", body: "x", typeflag: tar.TypeReg}},
		"absolute":        {manifest(1), jsonl, {name: "/abs", body: "x", typeflag: tar.TypeReg}},
		"symlink":         {manifest(1), jsonl, {name: "session/link", typeflag: tar.TypeSymlink}},
		"unknown entry":   {manifest(1), jsonl, {name: "other.txt", body: "x", typeflag: tar.TypeReg}},
		"manifest second": {jsonl, manifest(1)},
		"future format":   {manifest(2), jsonl},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if _, _, err := NewStore(root).Install(handBundle(t, entries...), "/x/y"); err == nil {
				t.Fatal("expected an error")
			}
			if files := regularFiles(t, root); len(files) > 0 {
				t.Errorf("left files behind: %v", files)
			}
		})
	}
}

func TestInstallSizeCap(t *testing.T) {
	bundle := packFixture(t, fixtureLines, nil)
	gz, err := gzip.NewReader(bytes.NewReader(bundle.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	if _, err := raw.ReadFrom(gz); err != nil {
		t.Fatal(err)
	}
	old := maxBytes
	t.Cleanup(func() { maxBytes = old })

	for name, limit := range map[string]int64{"mid-stream": 600, "one byte over": int64(raw.Len()) - 1} {
		t.Run(name, func(t *testing.T) {
			maxBytes = limit
			root := t.TempDir()
			if _, _, err := NewStore(root).Install(bytes.NewReader(bundle.Bytes()), "/x/y"); err == nil {
				t.Fatal("expected a size error")
			}
			if files := regularFiles(t, root); len(files) > 0 {
				t.Errorf("left files behind: %v", files)
			}
		})
	}
}
