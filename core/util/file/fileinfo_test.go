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

// +build !windows

//https://github.com/elastic/beats/blob/master/LICENSE
//Licensed under the Apache License, Version 2.0 (the "License");

package file

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestStat(t *testing.T) {
	f, err := ioutil.TempFile("", "teststat")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	link := filepath.Join(os.TempDir(), "teststat-link")
	if err := os.Symlink(f.Name(), link); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(link)

	info, err := Stat(link)
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, info.Mode().IsRegular())

	uid, err := info.UID()
	if err != nil {
		t.Fatal(err)
	}
	assert.EqualValues(t, os.Geteuid(), uid)

	gid, err := info.GID()
	if err != nil {
		t.Fatal(err)
	}
	assert.EqualValues(t, os.Getegid(), gid)
}

func TestLstat(t *testing.T) {
	link := filepath.Join(os.TempDir(), "link")
	if err := os.Symlink("dummy", link); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(link)

	info, err := Lstat(link)
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, info.Mode()&os.ModeSymlink > 0)

	uid, err := info.UID()
	if err != nil {
		t.Fatal(err)
	}
	assert.EqualValues(t, os.Geteuid(), uid)

	gid, err := info.GID()
	if err != nil {
		t.Fatal(err)
	}
	assert.EqualValues(t, os.Getegid(), gid)
}
