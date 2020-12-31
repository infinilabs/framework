// +build !windows

package util

import (
	"os"
	"os/exec"
	"os/user"
	"strings"
)

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
