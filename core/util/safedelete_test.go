/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsDangerousPath(t *testing.T) {
	dangerous := []string{
		"/", "/etc", "/etc/passwd", "/etc/conf.d/x", "/usr", "/usr/local/foo",
		"/bin", "/bin/ls", "/root", "/root/x", "/proc", "/sys/kernel",
		"/var", "/opt", "/home", "/tmp", // bare roots are not deletable
	}
	for _, p := range dangerous {
		if !IsDangerousPath(p) {
			t.Errorf("expected %s to be dangerous", p)
		}
	}
	safe := []string{
		"/var/lib/elasticsearch", "/opt/agent/data/x", "/srv/repo/snap",
		"/home/user/repodata", "/tmp/infini-migration-translog-x", "/data/transfer-repo",
	}
	for _, p := range safe {
		if IsDangerousPath(p) {
			t.Errorf("expected %s to be safe", p)
		}
	}
}

func TestIsPathWithin(t *testing.T) {
	if !IsPathWithin("/data/repo/a/b", "/data/repo") {
		t.Error("nested path should be within root")
	}
	if IsPathWithin("/data/repo", "/data/repo") {
		t.Error("target == root must NOT count as within (no self-delete)")
	}
	if IsPathWithin("/data/repo-evil/x", "/data/repo") {
		t.Error("prefix-sibling must not pass (classic HasPrefix bug)")
	}
	if IsPathWithin("/elsewhere/x", "/data/repo") {
		t.Error("outside path must not pass")
	}
}

func TestSafeRemoveAll(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "snapshots", "job1")
	if err := os.MkdirAll(inner, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "f"), []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}

	if err := SafeRemoveAll(inner, root); err != nil {
		t.Fatalf("legit delete failed: %v", err)
	}
	if _, err := os.Stat(inner); !os.IsNotExist(err) {
		t.Fatal("inner should be gone")
	}

	// outside the root
	outside := t.TempDir()
	if err := SafeRemoveAll(outside, root); err == nil {
		t.Fatal("delete outside allowed root must be rejected")
	}

	// the root itself
	if err := SafeRemoveAll(root, root); err == nil {
		t.Fatal("deleting the allowed root itself must be rejected")
	}

	// dangerous target
	if err := SafeRemoveAll("/etc/passwd", root); err == nil {
		t.Fatal("dangerous target must be rejected")
	}
	// dangerous root
	if err := SafeRemoveAll(filepath.Join(root, "x"), "/etc"); err == nil {
		t.Fatal("dangerous root must be rejected")
	}

	// symlink escape: link inside root pointing outside
	other := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(other, link); err != nil {
		t.Skip("symlink unavailable")
	}
	if err := SafeRemoveAll(link, root); err == nil {
		t.Fatal("symlink escaping the root must be rejected")
	}
}
