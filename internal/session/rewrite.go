package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// rewriter maps a sender's session onto the receiver's ID, folder and sidecar path.
type rewriter struct {
	oldID, newID   string
	oldCwd, newCwd string
	oldDir, newDir []byte
}

func newRewriter(m Manifest, newID, newCwd, newDir string) rewriter {
	return rewriter{
		oldID: m.SessionID, newID: newID,
		oldCwd: m.Cwd, newCwd: newCwd,
		oldDir: jsonString(m.SessionDir), newDir: jsonString(newDir),
	}
}

// copy rewrites r line by line into w, keeping each line's original newline.
func (rw rewriter) copy(w io.Writer, r io.Reader) error {
	br := bufio.NewReader(r)
	bw := bufio.NewWriter(w)
	for {
		line, readErr := br.ReadBytes('\n')
		body, nl := bytes.CutSuffix(line, []byte("\n"))
		if out, keep := rw.line(body); keep && (len(line) > 0) {
			_, _ = bw.Write(out)
			if nl {
				_ = bw.WriteByte('\n')
			}
		}
		if errors.Is(readErr, io.EOF) {
			return bw.Flush()
		}
		if readErr != nil {
			return readErr
		}
	}
}

// line rewrites one transcript line; keep is false for lines that must not reach the receiver.
func (rw rewriter) line(b []byte) (out []byte, keep bool) {
	b = bytes.ReplaceAll(b, rw.oldDir, rw.newDir)
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil || fields == nil {
		return b, true
	}
	if t, _ := stringField(fields, "type"); t == "permission-mode" {
		return nil, false
	}
	delete(fields, "permissionMode")
	if c, ok := stringField(fields, "cwd"); ok && rw.oldCwd != "" && (c == rw.oldCwd || strings.HasPrefix(c, rw.oldCwd+"/")) {
		fields["cwd"] = quoted(rw.newCwd + c[len(rw.oldCwd):])
	}
	for _, k := range []string{"sessionId", "session_id"} {
		if v, ok := stringField(fields, k); ok && v == rw.oldID {
			fields[k] = quoted(rw.newID)
		}
	}
	return encode(fields), true
}

func stringField(fields map[string]json.RawMessage, key string) (string, bool) {
	var s string
	raw, ok := fields[key]
	if !ok || json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}

// encode marshals v without HTML escaping and without a trailing newline.
func encode(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

func quoted(s string) json.RawMessage {
	return encode(s)
}

// jsonString is s as it appears inside a JSON string literal, without quotes.
func jsonString(s string) []byte {
	return bytes.Trim(encode(s), `"`)
}
