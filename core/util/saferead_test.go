/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestIsSystemReadPathUnixPathWithColon(t *testing.T) {
	// second-char-colon unix paths must stay on the unix branch: not
	// system paths themselves, and real system paths still denied
	if IsSystemReadPath("/t:mp/logs/server.log") {
		t.Error("expected colon-containing unix path to be readable")
	}
	if !IsSystemReadPath("/etc/passwd") {
		t.Error("expected /etc/passwd to stay denied")
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

func TestReadGuardContainsUnder(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "gc")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	guard, err := NewReadGuard(dir)
	if err != nil {
		t.Fatal(err)
	}

	if !guard.ContainsUnder(dir) {
		t.Error("expected root itself to match")
	}
	if !guard.ContainsUnder(sub) {
		t.Error("expected subdirectory of the root to match")
	}
	if guard.ContainsUnder(dir + "-sibling") {
		t.Error("expected prefix sibling to be rejected")
	}
	if guard.ContainsUnder("/etc") {
		t.Error("expected outside path to be rejected")
	}

	// base may be a subdirectory; the root stays the trust boundary
	logFile := filepath.Join(dir, "server.log")
	if err := os.WriteFile(logFile, []byte("line"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, err := guard.ResolveUnder(sub, "../server.log"); err != nil || got != canonicalPath(logFile) {
		t.Errorf("expected escape from base but not root to resolve, got %v %v", got, err)
	}
	if _, err := guard.ResolveUnder(sub, "../../../etc/passwd"); err == nil {
		t.Error("expected escape out of the root to be rejected")
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
func TestValidateReadRoot(t *testing.T) {
	dir := t.TempDir()
	denied := []string{
		"/", "/etc", "/etc/nginx", "/root",
		"/home", "/var", "/opt", // wholesale top-level trees
		"/var/log/../../etc", // traversal into a system tree
	}
	for _, p := range denied {
		if err := ValidateReadRoot(p); err == nil {
			t.Errorf("expected [%s] to be rejected", p)
		}
	}
	allowed := []string{
		"/var/log",
		"/var/log/elasticsearch",
		"/opt/myapp/logs",
		filepath.Join(dir, "logs"),
		filepath.Join(dir, "not-created-yet"), // existence is the caller's concern
	}
	for _, p := range allowed {
		if err := ValidateReadRoot(p); err != nil {
			t.Errorf("expected [%s] to pass: %v", p, err)
		}
	}
}

func TestIsConfinedUnder(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.log")

	if !IsConfinedUnder(root, root) {
		t.Error("the root itself must be confined")
	}
	if !IsConfinedUnder(root, filepath.Join(root, "app", "a.log")) {
		t.Error("file under the root must be confined")
	}
	if !IsConfinedUnder(root, filepath.Join(root, "sub", "..", "a.log")) {
		t.Error("cleaned traversal inside the root must be confined")
	}
	if IsConfinedUnder(root, outside) {
		t.Error("file outside the root must not be confined")
	}
	if IsConfinedUnder(root, root+"-escape") {
		t.Error("sibling sharing the prefix must not pass as confined")
	}

	link := filepath.Join(root, "jump.log")
	// the target must exist, otherwise EvalSymlinks cannot resolve the
	// link and the deepest-ancestor fallback would compare the raw link
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if IsConfinedUnder(root, link) {
		t.Error("symlink escaping the root must not be confined")
	}
}

// Helper function to create a real log file for guard authorization and resolution tests.
//
// These tests need actual filesystem entries so they exercise the guard's path
// resolution and file checks without mocked matchers.
func makeReadLog(t *testing.T, file string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("server log\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return canonicalReadPath(file)
}

// Wildcard construction cleans path components, validates prefixes, and copies root descriptions.
func TestNewReadGuardWildcardRoots(t *testing.T) {
	base := canonicalReadPath(t.TempDir())
	pattern := filepath.ToSlash(base) + "/node-*/logs"
	guard, err := NewReadGuard(pattern)
	if err != nil {
		t.Fatal(err)
	}
	roots := guard.Roots()
	if len(roots) != 1 || roots[0] != filepath.FromSlash(pattern) {
		t.Fatalf("unexpected root descriptions: %v", roots)
	}
	roots[0] = "changed"
	if guard.Roots()[0] != filepath.FromSlash(pattern) {
		t.Fatal("Roots exposed guard storage")
	}

	file := makeReadLog(t, filepath.Join(base, "node-a", "logs", "server.log"))
	t.Chdir(base)
	for _, rule := range []string{
		"./node-*/logs", "*/logs", "node-*/logs/", "./node-*//./logs/",
		"./absent/../node-*/logs", "node-*/logs/../logs", "node-*/./logs",
		filepath.ToSlash(base) + "/node-*/logs/",
	} {
		t.Run(rule, func(t *testing.T) {
			g, err := NewReadGuard(rule)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Dir(file)
			if !g.Contains(dir) || !g.ContainsUnder(dir) {
				t.Fatalf("cleaned rule did not authorize %s", dir)
			}
			if got, err := g.ResolveUnder(dir, "server.log"); err != nil || got != file {
				t.Fatalf("cleaned rule resolve = %q, %v", got, err)
			}
		})
	}

	fixedLog := makeReadLog(t, filepath.Join(base, "logs", "server.log"))
	for _, rule := range []string{"node-*/../logs", "**/../logs"} {
		g, err := NewReadGuard(rule)
		if err != nil {
			t.Fatal(err)
		}
		if roots := g.Roots(); len(roots) != 1 || roots[0] != filepath.Dir(fixedLog) {
			t.Errorf("collapsed rule roots = %v", roots)
		}
		if !g.Contains(filepath.Dir(fixedLog)) {
			t.Errorf("%q did not collapse to the literal logs root", rule)
		}
		if got, err := g.ResolveUnder(filepath.Dir(fixedLog), "server.log"); err != nil || got != fixedLog {
			t.Errorf("collapsed rule resolve = %q, %v", got, err)
		}
	}
	badFile := makeReadLog(t, filepath.Join(base, "not-directory"))
	for _, rule := range []string{
		filepath.ToSlash(base) + "/missing/*/logs",
		filepath.ToSlash(badFile) + "/*/logs",
		filepath.ToSlash(base) + "/node-[ab/logs",
	} {
		if _, err := NewReadGuard(rule); err == nil {
			t.Errorf("expected invalid rule %q to fail", rule)
		}
	}
	if runtime.GOOS != "windows" {
		for _, rule := range []string{"/etc/*/logs", "/home/*/logs", "/srv/node-*/..", "/node-*/../.."} {
			if _, err := NewReadGuard(rule); err == nil {
				t.Errorf("expected unsafe cleaned root %q to fail", rule)
			}
		}
		rooted, err := NewReadGuard("/../../" + strings.TrimLeft(filepath.ToSlash(base), "/") + "/node-*/logs")
		if err != nil || !rooted.Contains(filepath.Dir(file)) {
			t.Fatalf("root anchor was not preserved: %v", err)
		}
	}

	special := filepath.Join(base, "cwd[ab]{cd}")
	specialFile := makeReadLog(t, filepath.Join(special, "node-a", "logs", "server.log"))
	t.Chdir(special)
	g, err := NewReadGuard("./node-*/logs")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := g.ResolveUnder(filepath.Dir(specialFile), "server.log"); err != nil || got != specialFile {
		t.Fatalf("working directory was interpreted as glob syntax: %q, %v", got, err)
	}
	if g.Contains(filepath.Join(base, "cwdacd", "node-a", "logs")) {
		t.Fatal("literal working directory broadened authorization")
	}
}

// Wildcard matching preserves component depth, branch cleanup, boundaries, and rule unions.
func TestReadGuardWildcardMatching(t *testing.T) {
	base := canonicalReadPath(t.TempDir())
	paths := []string{
		"node-a/logs", "node-b/logs", "node-c/logs", "node-aa/logs",
		"node-a/extra/logs", "node-a/logs/gc/archive", "node-a/logs-backup",
		"node-a/config", "a/logs", "logs", "deep/a/logs", "deep/b/logs",
	}
	for _, path := range paths {
		makeReadLog(t, filepath.Join(base, filepath.FromSlash(path), "server.log"))
	}
	for _, tc := range []struct {
		rule  string
		path  string
		root  bool
		under bool
	}{
		{"node-*/logs", "node-a/logs", true, true},
		{"node-*/logs", "node-aa/logs", true, true},
		{"node-?/logs", "node-a/logs", true, true},
		{"node-?/logs", "node-aa/logs", false, false},
		{"node-*/logs", "node-a/extra/logs", false, false},
		{"node-?/logs", "node-a/extra/logs", false, false},
		{"node-**/logs", "node-a/extra/logs", true, true},
		{"node-*/logs", "node-a/logs/gc/archive", false, true},
		{"node-*/logs", "node-a/logs-backup", false, false},
		{"node-*/logs", "node-a/config", false, false},
		{"node-[ab]/logs", "node-a/logs", true, true},
		{"node-[ab]/logs", "node-c/logs", false, false},
		{"node-[a-c]/logs", "node-c/logs", true, true},
		{"node-[!ab]/logs", "node-c/logs", true, true},
		{"node-[!ab]/logs", "node-a/logs", false, false},
		{"{node-a,node-b}/logs", "node-b/logs", true, true},
		{"{node-a,node-b}/logs", "node-c/logs", false, false},
		{"node-{a,b}/logs", "node-a/logs", true, true},
		{"{a/b,c}/../logs", "a/logs", true, true},
		{"{a/b,c}/../logs", "logs", true, true},
		{"{a/b,c}/../logs", "node-a/logs", false, false},
		{"{deep/{a,b},node-a}/logs", "deep/b/logs", true, true},
		{"{no,node-}{a,b}/logs", "node-b/logs", true, true},
		{"node-a[!x]logs", "node-a/logs", true, true},
	} {
		t.Run(tc.rule+":"+tc.path, func(t *testing.T) {
			g, err := NewReadGuard(filepath.ToSlash(base) + "/" + tc.rule)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(base, filepath.FromSlash(tc.path))
			if got := g.Contains(dir); got != tc.root {
				t.Errorf("Contains = %v, expected %v", got, tc.root)
			}
			if got := g.ContainsUnder(dir); got != tc.under {
				t.Errorf("ContainsUnder = %v, expected %v", got, tc.under)
			}
			if tc.root {
				file := filepath.Join(dir, "server.log")
				if got, err := g.ResolveUnder(dir, "server.log"); err != nil || got != file {
					t.Errorf("ResolveUnder = %q, %v", got, err)
				}
			}
		})
	}
	literal := filepath.Join(base, "logs")
	wildcard := filepath.ToSlash(base) + "/node-*/logs"
	for _, rules := range [][]string{{literal, wildcard}, {wildcard, literal}} {
		g, err := NewReadGuard(rules...)
		if err != nil {
			t.Fatal(err)
		}
		if !g.Contains(literal) || !g.Contains(filepath.Join(base, "node-a", "logs")) {
			t.Error("rule order changed the authorization union")
		}
	}
}

// A slash character class retains glob semantics instead of becoming a path cleanup separator.
func TestReadGuardWildcardSeparatorClass(t *testing.T) {
	base := canonicalReadPath(t.TempDir())
	file := makeReadLog(t, filepath.Join(base, "node-a", "logs", "server.log"))
	for _, tc := range []struct {
		rule    string
		allowed bool
	}{
		{"node-a/[/]logs", false},
		{"node-a[/]logs", true},
	} {
		t.Run(tc.rule, func(t *testing.T) {
			g, err := NewReadGuard(filepath.ToSlash(base) + "/" + tc.rule)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Dir(file)
			if got := g.Contains(dir); got != tc.allowed {
				t.Errorf("Contains = %v, expected %v", got, tc.allowed)
			}
			if got := g.ContainsUnder(dir); got != tc.allowed {
				t.Errorf("ContainsUnder = %v, expected %v", got, tc.allowed)
			}
			resolved, err := g.ResolveUnder(dir, "server.log")
			if tc.allowed {
				if err != nil || resolved != file {
					t.Errorf("ResolveUnder = %q, %v", resolved, err)
				}
			} else if err == nil {
				t.Error("separator class authorized a path with a missing separator")
			}
		})
	}
}

// Unix escapes preserve literal metacharacters and backslashes beside real wildcard components.
func TestReadGuardWildcardEscaping(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses backslashes as path separators")
	}
	base := canonicalReadPath(t.TempDir())
	for _, tc := range []struct{ name, escaped, adjacent string }{
		{"node*", `node\*`, "node-a"}, {"node?", `node\?`, "nodea"},
		{"node[ab]", `node\[ab\]`, "nodea"}, {"node{a,b}", `node\{a,b\}`, "nodea"},
		{`node\back`, `node\\back`, "nodeback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			literal := makeReadLog(t, filepath.Join(base, tc.name, "logs", "server.log"))
			for _, suffix := range []string{"/logs", "/*/logs"} {
				file := literal
				adjacentDir := filepath.Join(base, tc.adjacent, "logs")
				if suffix == "/*/logs" {
					file = makeReadLog(t, filepath.Join(base, tc.name, "child", "logs", "server.log"))
					adjacentDir = filepath.Join(base, tc.adjacent, "child", "logs")
				}
				makeReadLog(t, filepath.Join(adjacentDir, "server.log"))
				g, err := NewReadGuard(base + "/" + tc.escaped + suffix)
				if err != nil {
					t.Fatal(err)
				}
				if got, err := g.ResolveUnder(filepath.Dir(file), "server.log"); err != nil || got != file {
					t.Fatalf("literal escape resolve = %q, %v", got, err)
				}
				if g.Contains(adjacentDir) || g.ContainsUnder(adjacentDir) {
					t.Fatalf("escaped rule %q authorized adjacent directory %q", tc.escaped+suffix, adjacentDir)
				}
				if _, err := g.ResolveUnder(adjacentDir, "server.log"); err == nil {
					t.Fatalf("escaped rule %q authorized an adjacent file", tc.escaped+suffix)
				}
			}
		})
	}
}

// Windows keeps both separators, cleans dot components, and accepts singleton literal classes.
func TestReadGuardWildcardWindowsSeparators(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("requires Windows filepath semantics")
	}
	base := canonicalReadPath(t.TempDir())
	file := makeReadLog(t, filepath.Join(base, "node-a", "logs", "server.log"))
	rule := filepath.ToSlash(base) + "/node-*/logs"
	for _, pattern := range []string{
		rule, strings.ReplaceAll(rule, "/", `\`),
		filepath.FromSlash(filepath.ToSlash(base)) + `/node-*\logs`,
		filepath.FromSlash(filepath.ToSlash(base)) + `/node-*\.\logs`,
		filepath.FromSlash(filepath.ToSlash(base)) + `/node-*\absent\..\logs`,
		filepath.ToSlash(filepath.Dir(file)), filepath.Dir(file),
	} {
		g, err := NewReadGuard(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if !g.Contains(filepath.Dir(file)) {
			t.Errorf("Windows separator rule %q did not match", pattern)
		}
		if got, err := g.ResolveUnder(filepath.Dir(file), "server.log"); err != nil || got != file {
			t.Errorf("Windows resolve = %q, %v", got, err)
		}
	}
	bracketFile := makeReadLog(t, filepath.Join(base, "[node]", "logs", "server.log"))
	native := filepath.ToSlash(base) + "/[[]node]/logs"
	for _, pattern := range []string{native, strings.Replace(native, string(native[0]), "["+string(native[0])+"]", 1)} {
		g, err := NewReadGuard(pattern)
		if err != nil || !g.Contains(filepath.Dir(bracketFile)) {
			t.Errorf("literal Windows class %q failed: %v", pattern, err)
		}
	}
	braceFile := makeReadLog(t, filepath.Join(base, "{node}", "logs", "server.log"))
	braceGuard, err := NewReadGuard(filepath.ToSlash(base) + "/[{]node}/logs")
	if err != nil || !braceGuard.Contains(filepath.Dir(braceFile)) {
		t.Fatalf("literal Windows brace failed: %v", err)
	}
	other := makeReadLog(t, filepath.Join(base, "node-b", "logs", "server.log"))
	volume := filepath.ToSlash(filepath.VolumeName(base))
	tail := strings.TrimPrefix(filepath.ToSlash(base), volume)
	for _, pattern := range []string{
		volume + "{" + tail + "/node-a," + tail + "/node-b}/logs",
		strings.TrimSuffix(volume, ":") + "{:" + tail + "/node-a,:" + tail + "/node-b}/logs",
	} {
		g, err := NewReadGuard(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, logFile := range []string{file, other} {
			if got, err := g.ResolveUnder(filepath.Dir(logFile), "server.log"); err != nil || got != logFile {
				t.Errorf("Windows alternative %q resolve = %q, %v", pattern, got, err)
			}
		}
	}
}

// Actual file resolution rejects unauthorized bases, targets, missing files, and directories.
func TestReadGuardWildcardResolveUnder(t *testing.T) {
	base := canonicalReadPath(t.TempDir())
	file := makeReadLog(t, filepath.Join(base, "node-a", "logs", "server.log"))
	gc := filepath.Join(filepath.Dir(file), "gc")
	if err := os.Mkdir(gc, 0755); err != nil {
		t.Fatal(err)
	}
	outside := makeReadLog(t, filepath.Join(base, "node-a", "config", "secret"))
	g, err := NewReadGuard(filepath.ToSlash(base) + "/node-*/logs")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ dir, name string }{
		{filepath.Dir(file), "server.log"}, {filepath.Dir(file), file}, {gc, "../server.log"},
	} {
		if got, err := g.ResolveUnder(tc.dir, tc.name); err != nil || got != file {
			t.Errorf("authorized resolve = %q, %v", got, err)
		}
	}
	for _, tc := range []struct{ dir, name string }{
		{filepath.Dir(outside), file}, {filepath.Dir(file), outside},
		{filepath.Dir(file), "../config/secret"}, {filepath.Dir(file), "missing.log"},
		{filepath.Dir(file), "gc"},
	} {
		if _, err := g.ResolveUnder(tc.dir, tc.name); err == nil {
			t.Errorf("expected unauthorized or invalid file %q from %q to fail", tc.name, tc.dir)
		}
	}
}

// Canonical targets reject wildcard symlink aliases and escapes while literal roots retain links.
func TestReadGuardWildcardCanonicalPaths(t *testing.T) {
	base := canonicalReadPath(t.TempDir())
	file := makeReadLog(t, filepath.Join(base, "real", "node-a", "logs", "server.log"))
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(filepath.Join(base, "real"), alias); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	aliasRule, err := NewReadGuard(filepath.ToSlash(alias) + "/node-*/logs")
	if err != nil {
		t.Fatal(err)
	}
	aliasDir := filepath.Join(alias, "node-a", "logs")
	if aliasRule.ContainsUnder(aliasDir) {
		t.Fatal("wildcard alias was mapped to its symlink target")
	}
	if _, err := aliasRule.ResolveUnder(aliasDir, "server.log"); err == nil {
		t.Fatal("wildcard alias authorized a resolved file")
	}
	g, err := NewReadGuard(filepath.ToSlash(base) + "/real/node-*/logs")
	if err != nil {
		t.Fatal(err)
	}
	outside := makeReadLog(t, filepath.Join(base, "outside", "secret.log"))
	link := filepath.Join(filepath.Dir(file), "outside")
	if err := os.Symlink(filepath.Dir(outside), link); err != nil {
		t.Fatal(err)
	}
	if g.ContainsUnder(link) {
		t.Fatal("symlink directory escaped the wildcard roots")
	}
	if _, err := g.ResolveUnder(filepath.Dir(file), "outside/secret.log"); err == nil {
		t.Fatal("resolved symlink escaped the wildcard roots")
	}
	literal, err := NewReadGuard(aliasDir)
	if err != nil || !literal.Contains(filepath.Dir(file)) {
		t.Fatalf("literal symlink root behavior changed: %v", err)
	}
	union, err := NewReadGuard(filepath.ToSlash(base)+"/real/node-*/logs", link)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := union.ResolveUnder(filepath.Dir(file), "outside/secret.log"); err != nil || got != outside {
		t.Fatalf("explicit literal link did not extend the rule union: %q, %v", got, err)
	}
}

// Authorization permits nonexistent matching paths, but file resolution requires a real file.
func TestReadGuardWildcardNewDirectory(t *testing.T) {
	base := canonicalReadPath(t.TempDir())
	g, err := NewReadGuard(filepath.ToSlash(base) + "/node-*/logs")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, "node-new", "logs")
	for _, tc := range []struct {
		path  string
		root  bool
		under bool
	}{
		{dir, true, true},
		{filepath.Join(dir, "gc"), false, true},
		{filepath.Join(base, "node-new", "config"), false, false},
	} {
		if got := g.Contains(tc.path); got != tc.root {
			t.Errorf("Contains(%q) = %v, expected %v", tc.path, got, tc.root)
		}
		if got := g.ContainsUnder(tc.path); got != tc.under {
			t.Errorf("ContainsUnder(%q) = %v, expected %v", tc.path, got, tc.under)
		}
	}
	if _, err := g.ResolveUnder(dir, "server.log"); err == nil {
		t.Fatal("missing log unexpectedly resolved")
	}
	file := makeReadLog(t, filepath.Join(dir, "server.log"))
	if got, err := g.ResolveUnder(dir, "server.log"); err != nil || got != file {
		t.Fatalf("existing guard did not accept the new file: %q, %v", got, err)
	}
}
