// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

//go:build linux
// +build linux

package rotate

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func TestMaintainMode(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping in CI environment")
	}

	dir := makeTempDir("TestMaintainMode", t)
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	mode := os.FileMode(0600)
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, mode)
	isNil(err, t)
	f.Close()

	l := &RotateWriter{
		Filename:         filename,
		MaxRotationCount: 1,
		MaxFileSize:      100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	newFakeTime()

	err = l.Rotate()
	isNil(err, t)

	filename2 := backupFile(dir)
	info, err := os.Stat(filename)
	isNil(err, t)
	info2, err := os.Stat(filename2)
	isNil(err, t)
	equals(mode, info.Mode(), t)
	equals(mode, info2.Mode(), t)
}

func TestMaintainOwner(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping in CI environment")
	}
	fakeFS := newFakeFS()
	osChown = fakeFS.Chown
	osStat = fakeFS.Stat
	defer func() {
		osChown = os.Chown
		osStat = os.Stat
	}()
	dir := makeTempDir("TestMaintainOwner", t)
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
	isNil(err, t)
	f.Close()

	l := &RotateWriter{
		Filename:         filename,
		MaxRotationCount: 1,
		MaxFileSize:      100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	newFakeTime()

	err = l.Rotate()
	isNil(err, t)

	equals(555, fakeFS.files[filename].uid, t)
	equals(666, fakeFS.files[filename].gid, t)
}

func TestCompressMaintainMode(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping in CI environment")
	}

	dir := makeTempDir("TestCompressMaintainMode", t)
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	mode := os.FileMode(0600)
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, mode)
	isNil(err, t)
	f.Close()

	l := &RotateWriter{
		Compress:         true,
		Filename:         filename,
		MaxRotationCount: 1,
		MaxFileSize:      100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	newFakeTime()

	err = l.Rotate()
	isNil(err, t)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// a compressed version of the log file should now exist with the correct
	// mode.
	filename2 := backupFile(dir)
	info, err := os.Stat(filename)
	isNil(err, t)
	info2, err := os.Stat(filename2 + compressSuffix)
	isNil(err, t)
	equals(mode, info.Mode(), t)
	equals(mode, info2.Mode(), t)
}

func TestCompressMaintainOwner(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping in CI environment")
	}
	fakeFS := newFakeFS()
	osChown = fakeFS.Chown
	osStat = fakeFS.Stat
	defer func() {
		osChown = os.Chown
		osStat = os.Stat
	}()
	dir := makeTempDir("TestCompressMaintainOwner", t)
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
	isNil(err, t)
	f.Close()

	l := &RotateWriter{
		Compress:         true,
		Filename:         filename,
		MaxRotationCount: 1,
		MaxFileSize:      100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	newFakeTime()

	err = l.Rotate()
	isNil(err, t)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// a compressed version of the log file should now exist with the correct
	// owner.
	filename2 := backupFile(dir)
	equals(555, fakeFS.files[filename2+compressSuffix].uid, t)
	equals(666, fakeFS.files[filename2+compressSuffix].gid, t)
}

type fakeFile struct {
	uid int
	gid int
}

type fakeFS struct {
	files map[string]fakeFile
}

func newFakeFS() *fakeFS {
	return &fakeFS{files: make(map[string]fakeFile)}
}

func (fs *fakeFS) Chown(name string, uid, gid int) error {
	fs.files[name] = fakeFile{uid: uid, gid: gid}
	return nil
}

func (fs *fakeFS) Stat(name string) (os.FileInfo, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	stat := info.Sys().(*syscall.Stat_t)
	stat.Uid = 555
	stat.Gid = 666
	return info, nil
}
