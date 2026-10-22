//go:build windows

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Junction requests match versioned log roots and reject junctions that escape them.
func TestReadGuardWindowsJunction(t *testing.T) {
	base := canonicalReadPath(t.TempDir())
	t.Chdir(base)
	assets := filepath.Join(base, "data", "agent", "nodes", "node-a", "services", "easysearch", "service-a", "assets")
	home := filepath.Join(assets, "easysearch-2.5.0-windows-amd64")
	file := makeReadLog(t, filepath.Join(home, "logs", "server.log"))
	alias := filepath.Join(assets, "easysearch")
	makeDirectoryJunction(t, alias, home)
	logs := filepath.Join(alias, "logs")
	outside := makeReadLog(t, filepath.Join(base, "outside", "secret.log"))
	escape := filepath.Join(logs, "outside")
	makeDirectoryJunction(t, escape, filepath.Dir(outside))

	for _, tc := range []struct{ name, rule string }{
		{"relative wildcard", "./data/agent/nodes/*/services/easysearch/*/assets/easysearch-*/logs"},
		{"literal junction root", logs},
	} {
		t.Run(tc.name, func(t *testing.T) {
			guard, err := NewReadGuard(tc.rule)
			if err != nil {
				t.Fatal(err)
			}
			if !guard.Contains(logs) || !guard.ContainsUnder(logs) {
				t.Fatal("junction log directory did not match its real root")
			}
			resolved, err := guard.ResolveUnder(logs, "server.log")
			if err != nil || resolved != file {
				t.Fatalf("ResolveUnder = %q, %v; expected %q", resolved, err, file)
			}
			if content, err := os.ReadFile(resolved); err != nil || string(content) != "server log\n" {
				t.Fatalf("read resolved log = %q, %v", content, err)
			}
			missing := filepath.Join(logs, "gc", "new")
			if guard.Contains(missing) || !guard.ContainsUnder(missing) {
				t.Fatal("missing descendants lost their resolved junction ancestor")
			}
			if _, err := guard.ResolveUnder(logs, "gc/new/missing.log"); err == nil {
				t.Fatal("nonexistent log was accepted")
			}
			if guard.Contains(escape) || guard.ContainsUnder(escape) || guard.ContainsUnder(filepath.Join(escape, "missing")) {
				t.Fatal("junction escaped the allowed log roots")
			}
			if _, err := guard.ResolveUnder(logs, "outside/secret.log"); err == nil {
				t.Fatal("junction authorized a file outside the allowed log roots")
			}
		})
	}
}

// Junctions resolve to the same target paths, including nonexistent descendants.
func TestCanonicalPathWindowsJunction(t *testing.T) {
	base := canonicalPath(t.TempDir())
	target := filepath.Join(base, "versioned")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "current")
	makeDirectoryJunction(t, link, target)
	for _, suffix := range []string{"", "new/logs/server.log"} {
		path := filepath.Join(link, suffix)
		want := filepath.Join(target, suffix)
		if got := canonicalPath(path); got != want {
			t.Errorf("canonicalPath(%q) = %q, expected %q", path, got, want)
		}
	}
}

// Deletion follows junction targets within allowed roots and preserves rejected targets.
func TestSafeRemoveAllWindowsJunction(t *testing.T) {
	for _, scope := range []string{"inside", "outside", "root"} {
		t.Run(scope, func(t *testing.T) {
			root := canonicalPath(t.TempDir())
			target := root
			switch scope {
			case "inside":
				target = filepath.Join(root, "versioned")
			case "outside":
				target = filepath.Join(canonicalPath(t.TempDir()), "versioned")
			}
			if err := os.MkdirAll(target, 0700); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(target, "data.txt")
			if err := os.WriteFile(file, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(root, "current")
			makeDirectoryJunction(t, link, target)
			err := SafeRemoveAll(link, root)
			if scope == "inside" {
				if err != nil {
					t.Fatalf("delete allowed junction target: %v", err)
				}
				if _, err := os.Stat(target); !os.IsNotExist(err) {
					t.Fatalf("junction target still exists: %v", err)
				}
				if _, err := os.Lstat(link); err != nil {
					t.Fatalf("expected the junction itself to remain: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected junction target outside the root or equal to it to be rejected")
				}
				if content, err := os.ReadFile(file); err != nil || string(content) != "keep" {
					t.Fatalf("rejected target was modified: %q, %v", content, err)
				}
			}
		})
	}
}

// Helper function to create a real directory junction without symlink privileges.
func makeDirectoryJunction(t *testing.T, link, target string) {
	t.Helper()
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Fatalf("create junction %q to %q: %v: %s", link, target, err, output)
	}
	t.Cleanup(func() {
		if err := os.Remove(link); err != nil {
			t.Errorf("remove junction %q: %v", link, err)
		}
	})
}
