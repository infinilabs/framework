//go:build windows

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import "errors"

func mkFifo(path string) error {
	return errors.New("fifo is not supported on windows")
}
