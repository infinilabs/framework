/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package taskset

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/util"
	"os/exec"
)

func SetCPUAffinityList(pid int, affinityList string) {
	if !util.FileExists("/usr/bin/taskset") {
		return
	}

	cmd := exec.Command("taskset", "-a","-c","-p", affinityList, fmt.Sprintf("%d", pid))
	_,err:=cmd.Output()
	if err!=nil{
		log.Error(err)
	}
}

func ResetCPUAffinityList(pid int) {
	if !util.FileExists("/usr/bin/taskset") {
		return
	}

	cmd := exec.Command("taskset", "-c","-p","0-1000", fmt.Sprintf("%d", pid))
	_,err:=cmd.Output()
	if err!=nil{
		log.Error(err)
	}
}
