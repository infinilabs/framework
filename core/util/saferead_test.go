/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsSystemReadPath(t *testing.T) {
	denied := []string{
		"/",
		"/etc", "/etc/passwd", "/etc/ssl/private/server.key",
		"/bin/ls", "/sbin", "/boot/grub",
		"/usr/bin/ls", "/usr/sbin/x", "/usr/lib/x.so", "/usr/lib64/x.so",
		"/root/.ssh/id_rsa", "/kernel",
		"/System/Library", "/private/etc/hosts",
		"/proc/self/environ", "/sys/kernel", "/dev/sda", "/run/secrets/token",
		// traversal into a system tree
		"/var/log/../../../etc/passwd",
	}
	for _, p := range denied {
		if !IsSystemReadPath(p) {
			t.Errorf("expected [%s] to be a system read path", p)
		}
	}

	allowed := []string{
		"/var/log/elasticsearch",
		"/usr/share/easysearch/logs/server.log",
		"/opt/myapp/logs",
		"/home/user/logs/app.log",
		"/srv/service/logs",
	}
	for _, p := range allowed {
		if IsSystemReadPath(p) {
			t.Errorf("expected [%s] to be a readable location", p)
		}
	}
}

func TestIsSystemReadCanonicalWindows(t *testing.T) {
	denied := []string{
		"C:/Windows", "C:/Windows/System32/cmd.exe", "c:/windows/system32/x.dll",
		"C:/Program Files/WindowsApps/App",
	}
	for _, p := range denied {
		if !isSystemReadCanonical(p) {
			t.Errorf("expected [%s] to be a system read path", p)
		}
	}
	allowed := []string{
		"C:/Program Files/Elasticsearch/logs",
		"C:/elastic/logs/server.log",
		"C:/ProgramData/myapp/logs",
	}
	for _, p := range allowed {
		if isSystemReadCanonical(p) {
			t.Errorf("expected [%s] to be a readable location", p)
		}
	}
}

func TestNewReadGuardRootValidation(t *testing.T) {
	dir := t.TempDir()

	if _, err := NewReadGuard(dir); err != nil {
		t.Fatalf("expected temp dir to be a valid root: %v", err)
	}

	for _, bad := range []string{"/etc", "/usr/bin", "/root", "/"} {
		if _, err := NewReadGuard(bad); err == nil {
			t.Errorf("expected system path [%s] to be rejected as root", bad)
		}
	}
	for _, broad := range []string{"/usr", "/var", "/opt", "/home", "/tmp"} {
		if _, err := NewReadGuard(broad); err == nil {
			t.Errorf("expected broad root [%s] to be rejected", broad)
		}
	}

	file := filepath.Join(dir, "notadir.log")
	if err := os.WriteFile(file, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewReadGuard(file); err == nil {
		t.Error("expected non-directory root to be rejected")
	}
	missing := filepath.Join(dir, "does-not-exist")
	if _, err := NewReadGuard(missing); err == nil {
		t.Error("expected missing root to be rejected")
	}
	if _, err := NewReadGuard(); err == nil {
		t.Error("expected empty roots to be rejected")
	}
}

func TestReadGuardResolveUnder(t *testing.T) {
	dir := t.TempDir()
	guard, err := NewReadGuard(dir)
	if err != nil {
		t.Fatal(err)
	}

	logFile := canonicalPath(filepath.Join(dir, "server.log"))
	if err := os.WriteFile(logFile, []byte("line"), 0644); err != nil {
		t.Fatal(err)
	}

	if got, err := guard.ResolveUnder(dir, "server.log"); err != nil || got != logFile {
		t.Errorf("relative resolve failed: %v %v", got, err)
	}
	if got, err := guard.ResolveUnder(dir, logFile); err != nil || got != logFile {
		t.Errorf("absolute-inside resolve failed: %v %v", got, err)
	}

	for _, bad := range []string{
		"../../../etc/passwd",
		"/etc/passwd",
		"",
		".",
		"missing.log",
	} {
		if _, err := guard.ResolveUnder(dir, bad); err == nil {
			t.Errorf("expected file [%q] to be rejected", bad)
		}
	}

	if _, err := guard.ResolveUnder("/etc", "passwd"); err == nil {
		t.Error("expected base outside the roots to be rejected")
	}
}

func TestReadGuardRejectsNonRegularFiles(t *testing.T) {
	dir := t.TempDir()
	guard, err := NewReadGuard(dir)
	if err != nil {
		t.Fatal(err)
	}

	sub := filepath.Join(dir, "nested")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := guard.ResolveUnder(dir, "nested"); err == nil {
		t.Error("expected directory passed as file to be rejected")
	}

	fifo := filepath.Join(dir, "pipe.log")
	if err := mkFifo(fifo); err != nil {
		t.Skipf("cannot create fifo on this platform: %v", err)
	}
	if _, err := guard.ResolveUnder(dir, "pipe.log"); err == nil {
		t.Error("expected fifo to be rejected")
	}
}

func TestReadGuardSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	guard, err := NewReadGuard(dir)
	if err != nil {
		t.Fatal(err)
	}

	logFile := canonicalPath(filepath.Join(dir, "server.log"))
	if err := os.WriteFile(logFile, []byte("line"), 0644); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(dir, "etc-link")
	if err := os.Symlink("/etc", link); err != nil {
		t.Fatal(err)
	}
	if _, err := guard.ResolveUnder(dir, "etc-link/passwd"); err == nil {
		t.Error("expected symlink escape into /etc to be rejected")
	}

	// a root reached through its own symlink still canonicalizes to itself
	linkToRoot := dir + "-via-link"
	if err := os.Symlink(dir, linkToRoot); err != nil {
		t.Fatal(err)
	}
	if !guard.Contains(linkToRoot) {
		t.Error("expected symlinked root variant to be recognized")
	}
	if got, err := guard.ResolveUnder(linkToRoot, "server.log"); err != nil || got != logFile {
		t.Errorf("resolve via symlinked root failed: %v %v", got, err)
	}
}

func TestReadGuardPrefixSafety(t *testing.T) {
	base := t.TempDir()
	sibling, err := os.MkdirTemp(filepath.Dir(base), filepath.Base(base)+"-sibling")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(sibling)
	if strings.HasPrefix(sibling, base+string(filepath.Separator)) {
		t.Fatalf("test requires sibling outside base: %s vs %s", sibling, base)
	}

	guard, err := NewReadGuard(base)
	if err != nil {
		t.Fatal(err)
	}
	escape := filepath.Join(sibling, "secret.log")
	if err := os.WriteFile(escape, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := guard.ResolveUnder(base, escape); err == nil {
		t.Error("expected prefix-sibling path (base vs base-sibling) to be rejected")
	}
}

func TestIsTooBroadReadRoot(t *testing.T) {
	for _, broad := range []string{"/usr", "/var", "/private/var"} {
		if !isTooBroadReadRoot(broad) {
			t.Errorf("expected [%s] to be too broad", broad)
		}
	}
	if isTooBroadReadRoot("/var/log/myapp") {
		t.Error("expected specific subdir to be acceptable")
	}
}
