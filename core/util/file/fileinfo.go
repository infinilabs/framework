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

// Package file copied from github.com/elastic/beats
// https://github.com/elastic/beats/blob/master/LICENSE
// Licensed under the Apache License, Version 2.0 (the "License");
package file

import (
	"errors"
	"os"
)

// A FileInfo describes a file and is returned by Stat and Lstat.
type FileInfo interface {
	os.FileInfo
	UID() (int, error) // UID of the file owner. Returns an error on non-POSIX file systems.
	GID() (int, error) // GID of the file owner. Returns an error on non-POSIX file systems.
}

// Stat returns a FileInfo describing the named file.
// If there is an error, it will be of type *PathError.
func Stat(name string) (FileInfo, error) {
	return stat(name, os.Stat)
}

// Lstat returns a FileInfo describing the named file.
// If the file is a symbolic link, the returned FileInfo
// describes the symbolic link. Lstat makes no attempt to follow the link.
// If there is an error, it will be of type *PathError.
func Lstat(name string) (FileInfo, error) {
	return stat(name, os.Lstat)
}

type fileInfo struct {
	os.FileInfo
	uid *int
	gid *int
}

func (f fileInfo) UID() (int, error) {
	if f.uid == nil {
		return -1, errors.New("uid not implemented")
	}

	return *f.uid, nil
}

func (f fileInfo) GID() (int, error) {
	if f.gid == nil {
		return -1, errors.New("gid not implemented")
	}

	return *f.gid, nil
}
