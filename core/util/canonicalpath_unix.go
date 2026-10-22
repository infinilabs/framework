//go:build !windows

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import "path/filepath"

// Helper function to make paths absolute and resolve symbolic links.
//
// Read and deletion guards compare actual filesystem destinations. For missing
// targets, resolving the deepest existing ancestor preserves that comparison
// when the path contains a symbolic link followed by nonexistent components.
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
		if parent == dir {
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
