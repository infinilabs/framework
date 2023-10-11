package host

import (
	"bufio"
	"bytes"
	log "github.com/cihub/seelog"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

//  getProcessInfo
//  @Description: get all es(running) process info
//
func getProcessInfo() string {
	cmdErr = nil
	switch runtime.GOOS {
	case "windows":
		return getProcessInfoWindows()
	default:
		return getProcessInfoLinux()
	}
}

func getProcessInfoWindows() string {
	//wmic process GET ProcessId,Name,CommandLine | findStr "Des.path.home"
	cmd := []string{"wmic", "process", "GET", "ProcessId,Name,CommandLine", "findStr", "Des.path.home"}
	var stdout bytes.Buffer
	c1 := exec.Command(cmd[0], cmd[1], cmd[2], cmd[3])
	c2 := exec.Command(cmd[4], cmd[5])
	c2.Stdin, _ = c1.StdoutPipe()
	c2.Stdout = &stdout
	cmdRun(c2.Start)
	cmdRun(c1.Run)
	cmdRun(c2.Wait)
	if cmdErr != nil {
		log.Errorf("failed : \n %s", cmdErr)
		return ""
	}
	return stdout.String()
}

func getProcessInfoLinux() string {
	//ps -ef | grep -v grep | grep elastic | grep Des.path.home
	cmds := []string{"ps", "-ef", "grep", "-v", "grep", "grep", "elastic", "grep", "Des.path.home"}
	var stdout bytes.Buffer
	c1 := exec.Command(cmds[0], cmds[1])
	c2 := exec.Command(cmds[2], cmds[3], cmds[4])
	c3 := exec.Command(cmds[5], cmds[6])
	c4 := exec.Command(cmds[7], cmds[8])
	c2.Stdin, _ = c1.StdoutPipe()
	c3.Stdin, _ = c2.StdoutPipe()
	c4.Stdin, _ = c3.StdoutPipe()
	c4.Stdout = &stdout
	cmdRun(c4.Start)
	cmdRun(c3.Start)
	cmdRun(c2.Start)
	cmdRun(c1.Run)
	cmdRun(c2.Wait)
	cmdRun(c3.Wait)
	cmdRun(c4.Wait)
	if cmdErr != nil {
		log.Debugf("host.getProcessInfo: get process info failed : \n %s", cmdErr)
		return ""
	}
	return stdout.String()
}

var cmdErr error

func cmdRun(f func() error) {
	if cmdErr != nil {
		return
	}
	cmdErr = f()
}

func getPortByPid(pid string) []int {
	if pid == "" {
		return nil
	}
	cmdErr = nil
	switch runtime.GOOS {
	case "windows":
		return getPortByPidWindows(pid)
	default:
		return getPortByPidLinux(pid)
	}
}

func getPortByPidWindows(pid string) []int {
	//netstat -ano|findStr /V "127.0.0.1" | findStr "780"
	cmd := []string{"netstat", "-ano", "findStr", "/V", "127.0.0.1", "findStr", pid}
	var stdout bytes.Buffer
	c1 := exec.Command(cmd[0], cmd[1])
	c2 := exec.Command(cmd[2], cmd[3], cmd[4])
	c3 := exec.Command(cmd[5], cmd[6])
	c2.Stdin, _ = c1.StdoutPipe()
	c3.Stdin, _ = c2.StdoutPipe()
	c3.Stdout = &stdout
	cmdRun(c3.Start)
	cmdRun(c2.Start)
	cmdRun(c1.Run)
	cmdRun(c2.Wait)
	cmdRun(c3.Wait)
	if cmdErr != nil {
		log.Errorf("host.getPortByPid: get process info failed, %s", cmdErr)
		return nil
	}
	resultTemp := make(map[int]int)
	scanner := bufio.NewScanner(strings.NewReader(stdout.String()))
	for scanner.Scan() {
		content := scanner.Text()
		temps := strings.Split(content, " ")
		for _, str := range temps {
			if strings.Contains(str, ":") {
				temp2 := strings.Split(str, ":")
				if len(temp2) > 2 {
					continue
				}
				for _, str2 := range temp2 {
					if v, err := strconv.Atoi(str2); err == nil {
						if v == 0 {
							continue
						}
						resultTemp[v] = v
						break
					}
				}
			}
		}
	}

	var result []int
	for _, v := range resultTemp {
		result = append(result, v)
	}
	return result
}

func getPortByPidLinux(pid string) []int {
	//lsof -i -P | grep -i LISTEN | grep #port#
	cmd := []string{"lsof", "-i", "-P", "grep", "-i", "LISTEN", "grep", pid}
	var stdout bytes.Buffer
	c1 := exec.Command(cmd[0], cmd[1], cmd[2])
	c2 := exec.Command(cmd[3], cmd[4], cmd[5])
	c3 := exec.Command(cmd[6], cmd[7])
	c2.Stdin, _ = c1.StdoutPipe()
	c3.Stdin, _ = c2.StdoutPipe()
	c3.Stdout = &stdout
	cmdRun(c3.Start)
	cmdRun(c2.Start)
	cmdRun(c1.Run)
	cmdRun(c2.Wait)
	cmdRun(c3.Wait)
	if cmdErr != nil {
		log.Errorf("host.getPortByPid: get process info failed, %s", cmdErr)
		return nil
	}
	out := stdout.String()
	sc := bufio.NewScanner(strings.NewReader(out))
	retMap := make(map[int]int)
	for sc.Scan() {
		info := sc.Text()
		spls := strings.Split(info, " ")
		for _, str := range spls {
			if strings.Contains(str, ":") {
				port := strings.Split(str, ":")[1]
				retInt, err := strconv.Atoi(port)
				if err != nil {
					continue
				}
				retMap[retInt] = retInt
			}
		}
	}

	var ports []int
	for _, v := range retMap {
		ports = append(ports, v)
	}
	return ports
}
