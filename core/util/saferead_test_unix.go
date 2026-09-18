//go:build !windows

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import "syscall"

func mkFifo(path string) error {
	return syscall.Mkfifo(path, 0600)
}
