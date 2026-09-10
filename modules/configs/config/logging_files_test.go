/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package config

import (
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
