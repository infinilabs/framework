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

// Deletion safety: any removal driven by external input (a configured
// repository root, a control-plane-supplied target path) must go through
// SafeRemoveAll so a crafted path cannot turn the service into a
// general-purpose "rm -rf".

// neverTouch lists locations that must never be deleted, nor contain
// anything deleted by these helpers: the filesystem root and OS-critical
// trees. Data-friendly roots (/opt, /srv, /home, /mnt, /media, /tmp, /var)
// are intentionally absent — data directories commonly live there (e.g.
// /var/lib/elasticsearch); their top level is protected by neverDeleteRoot
// instead.
var neverTouch = map[string]bool{
	"/bin": true, "/sbin": true, "/boot": true, "/dev": true, "/etc": true,
	"/proc": true, "/sys": true, "/run": true, "/usr": true, "/lib": true,
	"/lib64": true, "/libx32": true, "/root": true, "/kernel": true,
	"/System": true, "/Library": true, "/private/etc": true,
}

// neverDeleteRoot lists directories that may contain data but must not be
// deleted as a whole (target == one of these, or used as the deletion root
// itself, is rejected).
var neverDeleteRoot = map[string]bool{
	"/var": true, "/opt": true, "/srv": true, "/home": true,
	"/mnt": true, "/media": true, "/tmp": true,
	// canonical forms of the symlinked roots on macOS
	"/private/var": true, "/private/tmp": true,
}

// canonicalPath absolutizes and symlink-resolves p. When p itself does not
// exist yet (the common case: cleaning a destination right before writing
// it), the deepest EXISTING ancestor is resolved instead, so a not-yet-
// created target under a symlinked root canonicalizes identically to the
// root — and a link jumping out of the allowed root cannot smuggle a
// target past the containment check either way.
func canonicalPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = filepath.Clean(p)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	rest := ""
	dir := abs
	for {
		parent := filepath.Dir(dir)
		if parent == dir { // reached "/"
			break
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		if resolved, err := filepath.EvalSymlinks(parent); err == nil {
			return filepath.Join(resolved, rest)
		}
		dir = parent
	}
	return abs
}

// IsDangerousPath reports whether p is a system-critical location that must
// not be deleted: the filesystem root, a neverTouch member or anything
// under it, or a bare neverDeleteRoot member (its CONTENT is fine).
func IsDangerousPath(p string) bool {
	c := canonicalPath(p)
	// macOS firmlinks: user data lives on the data volume also visible as
	// /System/Volumes/Data/... — canonicalizing there must not misclassify
	// user paths as system files. Map back to the firmlink view (/Users,
	// /home, /opt, ...) before the root checks.
	if strings.HasPrefix(c, "/System/Volumes/Data/") {
		c = c[len("/System/Volumes/Data"):]
	}
	if c == "/" || c == string(filepath.Separator) {
		return true
	}
	if neverTouch[c] {
		return true
	}
	if neverDeleteRoot[c] {
		return true
	}
	for d := range neverTouch {
		if strings.HasPrefix(c, d+"/") {
			return true
		}
	}
	return false
}

// IsPathWithin reports whether target is strictly BELOW root (target ==
// root does not count: deleting the root itself is a different operation
// and must be opted into explicitly). Lexical on cleaned absolute paths,
// with symlink resolution when the paths exist.
func IsPathWithin(target, root string) bool {
	t, r := canonicalPath(target), canonicalPath(root)
	if t == r {
		return false
	}
	return strings.HasPrefix(t, r+string(filepath.Separator))
}

// SafeRemoveAll removes target (file or tree, like os.RemoveAll) only when
// it is strictly within one of the given roots and is not a dangerous
// path. Every root must itself be non-dangerous; otherwise nothing is
// removed. The error message states which guard tripped, so misconfigured
// callers fail loudly instead of silently skipping cleanup.
func SafeRemoveAll(target string, roots ...string) error {
	t := canonicalPath(target)
	if IsDangerousPath(t) {
		return fmt.Errorf("refusing to delete dangerous path: %s", t)
	}
	ok := false
	for _, root := range roots {
		r := canonicalPath(root)
		if IsDangerousPath(r) {
			return fmt.Errorf("refusing to delete within dangerous root: %s", r)
		}
		if IsPathWithin(t, r) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("refusing to delete %s: not within any allowed root %v", t, roots)
	}
	return os.RemoveAll(t)
}
