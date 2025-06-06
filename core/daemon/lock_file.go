//go:build linux || darwin
// +build linux darwin

package daemon

import (
	"fmt"
	"os"
	"syscall"
)

var (
	// ErrWouldBlock indicates on locking pid-file by another process.
	ErrWouldBlock = syscall.EWOULDBLOCK
)

// LockFile wraps *os.File and provide functions for locking of files.
type LockFile struct {
	*os.File
}

// NewLockFile returns a new LockFile with the given File.
func NewLockFile(file *os.File) *LockFile {
	return &LockFile{file}
}

// CreatePidFile opens the named file, applies exclusive lock and writes
// current process id to file.
func CreatePidFile(name string, perm os.FileMode) (lock *LockFile, err error) {
	if lock, err = OpenLockFile(name, perm); err != nil {
		return
	}
	if err = lock.Lock(); err != nil {
		lock.Remove()
		return
	}
	if err = lock.WritePid(); err != nil {
		lock.Remove()
	}
	return
}

// OpenLockFile opens the named file with flags os.O_RDWR|os.O_CREATE and specified perm.
// If successful, function returns LockFile for opened file.
func OpenLockFile(name string, perm os.FileMode) (lock *LockFile, err error) {
	var file *os.File
	if file, err = os.OpenFile(name, os.O_RDWR|os.O_CREATE, perm); err == nil {
		lock = &LockFile{file}
	}
	return
}

// Lock apply exclusive lock on an open file. If file already locked, returns error.
func (file *LockFile) Lock() error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// Unlock remove exclusive lock on an open file.
func (file *LockFile) Unlock() error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}

// ReadPidFile reads process id from file with give name and returns pid.
// If unable read from a file, returns error.
func ReadPidFile(name string) (pid int, err error) {
	var file *os.File
	if file, err = os.OpenFile(name, os.O_RDONLY, 0640); err != nil {
		return
	}
	defer file.Close()

	lock := &LockFile{file}
	pid, err = lock.ReadPid()
	return
}

// WritePid writes current process id to an open file.
func (file *LockFile) WritePid() (err error) {
	if _, err = file.Seek(0, os.SEEK_SET); err != nil {
		return
	}
	var fileLen int
	if fileLen, err = fmt.Fprint(file, os.Getpid()); err != nil {
		return
	}
	if err = file.Truncate(int64(fileLen)); err != nil {
		return
	}
	err = file.Sync()
	return
}

// ReadPid reads process id from file and returns pid.
// If unable read from a file, returns error.
func (file *LockFile) ReadPid() (pid int, err error) {
	if _, err = file.Seek(0, os.SEEK_SET); err != nil {
		return
	}
	_, err = fmt.Fscan(file, &pid)
	return
}

// Remove removes lock, closes and removes an open file.
func (file *LockFile) Remove() error {
	defer file.Close()

	if err := file.Unlock(); err != nil {
		return err
	}

	name, err := GetFdName(file.Fd())
	if err != nil {
		return err
	}

	err = syscall.Unlink(name)
	return err
}

// GetFdName returns file name for given descriptor.
func GetFdName(fd uintptr) (name string, err error) {
	path := fmt.Sprintf("/proc/self/fd/%d", int(fd))

	var (
		fi os.FileInfo
		n  int
	)
	if fi, err = os.Lstat(path); err != nil {
		return
	}
	buf := make([]byte, fi.Size()+1)

	if n, err = syscall.Readlink(path, buf); err == nil {
		name = string(buf[:n])
	}
	return
}
