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

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package taskset

import (
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/util"
	"os/exec"
)

func SetCPUAffinityList(pid int, affinityList string) {
	if !util.FileExists("/usr/bin/taskset") {
		return
	}

	cmd := exec.Command("taskset", "-a", "-c", "-p", affinityList, fmt.Sprintf("%d", pid))
	_, err := cmd.Output()
	if err != nil {
		log.Error(err)
	}
}

func ResetCPUAffinityList(pid int) {
	if !util.FileExists("/usr/bin/taskset") {
		return
	}

	cmd := exec.Command("taskset", "-c", "-p", "0-1000", fmt.Sprintf("%d", pid))
	_, err := cmd.Output()
	if err != nil {
		log.Error(err)
	}
}
