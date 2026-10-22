//go:build windows

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// Helper function to make paths absolute and resolve symbolic links and junctions.
//
// Since Go 1.23, junctions no longer carry ModeSymlink, so filepath.EvalSymlinks
// cannot resolve them. Windows needs a separate implementation from Unix to
// resolve both symbolic links and junctions through file handles.
//
// Missing targets retain their suffix below the deepest resolvable ancestor.
func canonicalPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}
	for candidate, suffix := abs, ""; ; {
		// CreateFile needs extended paths to reach junctions beyond MAX_PATH.
		// Unlike os.Open, this direct Windows call does not add that prefix.
		extended := candidate
		if !strings.HasPrefix(extended, `\\?\`) && !strings.HasPrefix(extended, `\\.\`) {
			if strings.HasPrefix(extended, `\\`) {
				extended = `\\?\UNC\` + extended[2:]
			} else {
				extended = `\\?\` + extended
			}
		}
		name, err := windows.UTF16PtrFromString(extended)
		if err != nil {
			return abs
		}
		// Query path metadata without requiring file-content access or blocking
		// concurrent readers, writers, and deletion.
		handle, err := windows.CreateFile(name, 0,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
			nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if err == nil {
			buffer := make([]uint16, windows.MAX_PATH)
			for {
				n, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
				if err != nil {
					break
				}
				if n >= uint32(len(buffer)) {
					buffer = make([]uint16, n+1)
					continue
				}
				resolved := windows.UTF16ToString(buffer[:n])
				windows.CloseHandle(handle)
				// Guards compare normal DOS or UNC paths, without device prefixes.
				if unc, ok := strings.CutPrefix(resolved, `\\?\UNC\`); ok {
					resolved = `\\` + unc
				} else {
					resolved = strings.TrimPrefix(resolved, `\\?\`)
				}
				return filepath.Join(resolved, suffix)
			}
			windows.CloseHandle(handle)
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return abs
		}
		suffix = filepath.Join(filepath.Base(candidate), suffix)
		candidate = parent
	}
}
