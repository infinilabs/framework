/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package managed

import (
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	httprouter "infini.sh/framework/core/api/router"
)

func stubLogDir(t *testing.T, dir string) {
	t.Helper()
	orig := currentLogDir
	currentLogDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { currentLogDir = orig })
}

func callTail(t *testing.T, file string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/logging/tail?file="+url.QueryEscape(file), nil)
	w := httptest.NewRecorder()
	tailLogFileAction(w, req, httprouter.Params{})
	return w
}

func TestTailLogFileAction(t *testing.T) {
	dir := t.TempDir()
	stubLogDir(t, dir)

	logFile := filepath.Join(dir, "server.log")
	content := "first\nsecond\nthird\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	w := callTail(t, "server.log")
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "third") {
		t.Errorf("expected tailed content in response: %s", w.Body.String())
	}

	for _, bad := range []string{
		"../../../etc/passwd",
		"/etc/passwd",
		"missing.log",
		"",
	} {
		if w := callTail(t, bad); w.Code != 400 {
			t.Errorf("expected 400 for file [%q], got %d: %s", bad, w.Code, w.Body.String())
		}
	}

	link := filepath.Join(dir, "escape.log")
	if err := os.Symlink("/etc/passwd", link); err != nil {
		t.Fatal(err)
	}
	if w := callTail(t, "escape.log"); w.Code != 400 {
		t.Errorf("expected 400 for symlink escape, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListLogFilesActionRejectsSystemLogDir(t *testing.T) {
	stubLogDir(t, "/etc")

	req := httptest.NewRequest("GET", "/logging/files", nil)
	w := httptest.NewRecorder()
	listLogFilesAction(w, req, httprouter.Params{})
	if w.Code != 500 {
		t.Errorf("expected 500 for system path as log dir, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListLogFilesActionDepthResetsBetweenSiblings(t *testing.T) {
	dir := t.TempDir()
	stubLogDir(t, dir)

	// a deep chain first so a buggy depth counter stays high, then a
	// shallow sibling directory that must still be listed
	deep := dir
	for i := 0; i < logReadDirMaxDepth+2; i++ {
		deep = filepath.Join(deep, fmt.Sprintf("d%d", i))
	}
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(dir, "shallow")
	if err := os.Mkdir(sibling, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sibling, "server.log"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/logging/files", nil)
	w := httptest.NewRecorder()
	listLogFilesAction(w, req, httprouter.Params{})
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "shallow/server.log") {
		t.Errorf("expected shallow sibling file to be listed: %s", w.Body.String())
	}
}

func TestListLogFilesAction(t *testing.T) {
	dir := t.TempDir()
	stubLogDir(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "server.log"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "slowlog.json"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/logging/files", nil)
	w := httptest.NewRecorder()
	listLogFilesAction(w, req, httprouter.Params{})
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"server.log", "nested/slowlog.json"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected [%s] in listing: %s", want, body)
		}
	}
	if strings.Contains(body, "notes.txt") {
		t.Errorf("expected non-log file to be excluded: %s", body)
	}
}
func TestTailLogFileActionRejectsNonLogSuffix(t *testing.T) {
	dir := t.TempDir()
	stubLogDir(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "archive.log.gz"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if w := callTail(t, "archive.log.gz"); w.Code != 400 {
		t.Errorf("expected 400 for non-log suffix, got %d: %s", w.Code, w.Body.String())
	}
}

// shortReaderAt simulates a file that shrank between stat and read.
type shortReaderAt struct {
	data []byte
	err  error
}

func (r shortReaderAt) ReadAt(p []byte, off int64) (int, error) {
	copy(p, r.data)
	return len(r.data), r.err
}

func TestReadTailWindowShortRead(t *testing.T) {
	// partial read with ErrUnexpectedEOF: bytes read so far are kept,
	// the zero-filled remainder is not served
	buf, full, err := readTailWindow(shortReaderAt{data: []byte("hello"), err: io.ErrUnexpectedEOF}, 1<<20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if full {
		t.Error("short read must not report a full window")
	}
	if string(buf) != "hello" {
		t.Errorf("expected the bytes actually read, got %q", buf)
	}

	// zero-byte read with EOF: empty result, no NUL padding
	buf, full, err = readTailWindow(shortReaderAt{err: io.EOF}, int64(logTailMaxBytes))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if full || len(buf) != 0 {
		t.Errorf("expected empty non-full window, got len=%d full=%v", len(buf), full)
	}

	// any other error propagates
	if _, _, err := readTailWindow(shortReaderAt{err: os.ErrPermission}, 10); !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected the read error to propagate, got %v", err)
	}

	// empty file: no read at all
	buf, full, err = readTailWindow(shortReaderAt{err: io.EOF}, 0)
	if err != nil || full || len(buf) != 0 {
		t.Errorf("expected empty result for empty file, got len=%d full=%v err=%v", len(buf), full, err)
	}
}
