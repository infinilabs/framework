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

package util

import (
	"os"
	"os/exec"
	"os/user"
	"strings"
)

func IsRootUser() bool {
	user,err:=user.Current()
	if err!=nil{
		panic(err)
	}
	if user.Name=="root" || user.Username=="root"{
		return true
	}
	return false
}

func HasSudoPermission() bool {

	user,err:=user.Current()

	if err!=nil{
		panic(err)
	}

	cmd := exec.Command("groups", user.Username)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	data, err := cmd.Output()
	if err != nil {
		panic(err)
	}

	if strings.Contains(string(data),"root")||strings.Contains(string(data),"admin"){
		return true
	}else{
		return false
	}
}
