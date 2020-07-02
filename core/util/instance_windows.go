package util

import (
	"os/exec"
	"strconv"
)

// CheckProcessExists check if the pid is running
func CheckProcessExists(pid int) bool {
	cmd, _ := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid)).Output()
	output := string(cmd[:])
	if ContainStr(output, "PID") {
		return true
	} else {
		return false
	}
}
