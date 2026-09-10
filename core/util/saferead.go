/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Read-side file access control: any handler that serves file contents
// driven by external input (a caller-supplied directory, a file name from
// a query string) must resolve the path through a ReadGuard so the
// endpoint cannot be turned into a general-purpose file reader.
//
// Two layers, deny always wins:
//  1. IsSystemReadPath — OS-critical trees are never readable, even when
//     a whitelist root was misconfigured to include them.
//  2. ReadGuard — files must live strictly below one of the explicitly
//     allowed roots (symlink-resolved on both sides).

// systemReadPrefixes lists trees that never legitimately hold readable
// service logs, so serving reads from them is refused outright. It is
// deliberately narrower than safedelete's neverTouch: packaged products
// keep logs under /usr/share/<product>/logs (RPM/DEB installs) and
// /var/log, /opt, /home are standard log locations — those stay
// reachable, constrained by the whitelist instead.
var systemReadPrefixes = []string{
	"/bin", "/sbin", "/boot", "/dev", "/etc", "/proc", "/sys", "/run",
	"/lib", "/lib64", "/libx32", "/root", "/kernel",
	"/System", "/private/etc",
	"/usr/bin", "/usr/sbin", "/usr/lib", "/usr/lib64", "/usr/libx32",
}

// windowsSystemReadPrefixes are matched against the volume-relative part
// of a drive path (C:/Windows/... -> /Windows/...) case-insensitively.
var windowsSystemReadPrefixes = []string{
	"/Windows", "/Program Files/WindowsApps",
}

// tooBroadReadRoots lists top-level trees that hold far more than logs;
// whitelisting one of them verbatim (instead of a specific subdirectory)
// is treated as a misconfiguration and rejected. Their contents remain
// reachable through narrower roots.
var tooBroadReadRoots = []string{
	"/usr", "/var", "/opt", "/srv", "/home", "/mnt", "/media", "/tmp",
	"/private/var", "/private/tmp",
}

// IsSystemReadPath reports whether p is an OS-critical location that must
// never be served by a file-reading API: the filesystem root, a
// systemReadPrefixes member or anything under it (drive roots and the
// listed Windows trees on Windows).
func IsSystemReadPath(p string) bool {
	return isSystemReadCanonical(canonicalReadPath(p))
}

// canonicalReadPath canonicalizes p and maps macOS firmlink paths
// (/System/Volumes/Data/...) back to their firmlink view (/home, /opt,
// ...) so every guard layer compares the same spelling.
func canonicalReadPath(p string) string {
	c := canonicalPath(p)
	if strings.HasPrefix(c, "/System/Volumes/Data/") {
		c = c[len("/System/Volumes/Data"):]
	}
	return c
}

// isSystemReadCanonical applies the deny checks to an already-canonical
// path so the Windows rules stay testable on every platform.
func isSystemReadCanonical(c string) bool {
	if c == "/" || c == string(filepath.Separator) {
		return true
	}
	// drive-qualified path (Windows): compare the volume-relative part.
	// The separator check keeps unix paths whose second character happens
	// to be ':' (e.g. /t:mp) on the unix branch.
	if len(c) > 2 && c[1] == ':' && (c[2] == '/' || c[2] == '\\') {
		c = filepath.ToSlash(c)
		if len(c) <= 3 { // bare drive root, e.g. C:/
			return true
		}
		rel := strings.ToLower(c[2:]) // /windows/system32
		for _, prefix := range windowsSystemReadPrefixes {
			prefix = strings.ToLower(prefix)
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				return true
			}
		}
		return false
	}
	for _, prefix := range systemReadPrefixes {
		if c == prefix || strings.HasPrefix(c, prefix+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// isTooBroadReadRoot reports whether an already-canonical directory is a
// wholesale top-level tree rather than a specific log location.
func isTooBroadReadRoot(c string) bool {
	for _, broad := range tooBroadReadRoots {
		if c == broad {
			return true
		}
	}
	return false
}

// ReadGuard confines file reads to a fixed set of allowed roots. Roots
// are canonicalized (absolute + symlink-resolved) once at construction;
// every later ResolveUnder call re-resolves the target the same way so a
// symlink cannot smuggle a path outside the roots.
type ReadGuard struct {
	roots []string
}

// NewReadGuard validates and canonicalizes the allowed roots. Every root
// must exist, be a directory, be a specific-enough location (not a
// wholesale /usr or /var), and not be a system path — a bad root fails
// loudly instead of being silently dropped.
func NewReadGuard(roots ...string) (*ReadGuard, error) {
	g := &ReadGuard{}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		c := canonicalReadPath(root)
		if IsSystemReadPath(c) {
			return nil, fmt.Errorf("refusing to serve reads from system path: %s", c)
		}
		if isTooBroadReadRoot(c) {
			return nil, fmt.Errorf("allowed path %s is too broad, whitelist a specific log directory instead", c)
		}
		fi, err := os.Stat(c)
		if err != nil {
			return nil, fmt.Errorf("allowed path %s is not accessible: %v", c, err)
		}
		if !fi.IsDir() {
			return nil, fmt.Errorf("allowed path %s is not a directory", c)
		}
		g.roots = append(g.roots, c)
	}
	if len(g.roots) == 0 {
		return nil, fmt.Errorf("no allowed read paths configured")
	}
	return g, nil
}

// Roots returns the canonical allowed roots.
func (g *ReadGuard) Roots() []string {
	return append([]string(nil), g.roots...)
}

// Contains reports whether path canonicalizes to exactly one of the
// allowed roots.
func (g *ReadGuard) Contains(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	c := canonicalReadPath(path)
	for _, r := range g.roots {
		if c == r {
			return true
		}
	}
	return false
}

// ContainsUnder reports whether path canonicalizes to one of the allowed
// roots or to a location strictly below one of them — the check for
// caller-supplied base directories that may be a subdirectory of a
// whitelisted root (e.g. a GC log dir configured under path.logs).
func (g *ReadGuard) ContainsUnder(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	c := canonicalReadPath(path)
	for _, r := range g.roots {
		if c == r || strings.HasPrefix(c, r+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// ResolveUnder resolves file against base (which must be an allowed root
// or live below one) and returns its canonical path when it lands strictly
// below an allowed root, outside every system tree, and is an existing
// regular file (so devices, fifos and sockets can never be served).
// file may be relative (joined to base) or absolute (accepted only when
// it still resolves inside the roots); the roots — not base — remain the
// trust boundary, so a file name escaping base but staying inside the
// root is still served.
func (g *ReadGuard) ResolveUnder(base, file string) (string, error) {
	if strings.TrimSpace(file) == "" {
		return "", fmt.Errorf("file is required")
	}
	if !g.ContainsUnder(base) {
		return "", fmt.Errorf("path [%v] is not an allowed directory", base)
	}
	target := file
	if !filepath.IsAbs(target) {
		target = filepath.Join(canonicalReadPath(base), target)
	}
	c := canonicalReadPath(target)
	if IsSystemReadPath(c) {
		return "", fmt.Errorf("path [%v] is a system path", file)
	}
	contained := false
	for _, r := range g.roots {
		if strings.HasPrefix(c, r+string(filepath.Separator)) {
			contained = true
			break
		}
	}
	if !contained {
		return "", fmt.Errorf("file [%v] is outside of the allowed directories", file)
	}
	fi, err := os.Stat(c)
	if err != nil {
		return "", fmt.Errorf("cannot access file [%v]: %v", file, err)
	}
	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("[%v] is not a regular file", file)
	}
	return c, nil
}
