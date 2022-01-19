// +build windows

/*
[nsq]: https://github.com/nsqio/nsq
Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
 [nsq]: https://github.com/nsqio/nsq
*/

package util

import (
	"syscall"
	"unsafe"
)

var (
	modkernel32     = syscall.NewLazyDLL("kernel32.dll")
	procMoveFileExW = modkernel32.NewProc("MoveFileExW")
)

const (
	MOVEFILE_REPLACE_EXISTING = 1
)

func moveFileEx(sourceFile, targetFile *uint16, flags uint32) error {
	ret, _, err := procMoveFileExW.Call(uintptr(unsafe.Pointer(sourceFile)), uintptr(unsafe.Pointer(targetFile)), uintptr(flags))
	if ret == 0 {
		if err != nil {
			return err
		}
		return syscall.EINVAL
	}
	return nil
}

func AtomicFileRename(sourceFile, targetFile string) error {
	lpReplacedFileName, err := syscall.UTF16PtrFromString(targetFile)
	if err != nil {
		return err
	}

	lpReplacementFileName, err := syscall.UTF16PtrFromString(sourceFile)
	if err != nil {
		return err
	}

	return moveFileEx(lpReplacementFileName, lpReplacedFileName, MOVEFILE_REPLACE_EXISTING)
}
