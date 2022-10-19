package util

import (
	"fmt"
	log "github.com/cihub/seelog"
	"os"
	"path"
	"path/filepath"
	"syscall"
)

var locked bool
var file string

func PidExists(pid int32) (bool, error) {
	if pid <= 0 {
		return false, fmt.Errorf("invalid pid %v", pid)
	}
	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return false, err
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true, nil
	}
	if err.Error() == "os: process already finished" {
		return false, nil
	}
	errno, ok := err.(syscall.Errno)
	if !ok {
		return false, err
	}
	switch errno {
	case syscall.ESRCH:
		return false, nil
	case syscall.EPERM:
		return true, nil
	}
	return false, err
}


// CheckInstanceLock make sure there is not a lock placed before check, and place a lock after check
func CheckInstanceLock(p string) {
	file = path.Join(p, ".lock")
	if FileExists(file) {
		pidBytes,err:=FileGetContent(file)
		if err==nil&&len(pidBytes)>0{
			pid,err:=ToInt(string(pidBytes))
			if err==nil&&pid>0{
				exists,err:=PidExists(int32(pid))
				if err==nil&&!exists{
					ClearInstanceLock()
					log.Debugf("pid [%v] exists, but process is gone, remove the lock file and continue",pid)
					return
				}
			}
			if pid>0 &&(pid==os.Getpid()||pid==os.Getppid()){
				log.Debugf("pid lock [%v] exists, but it's you, let's continue", pid)
				return
			}
		}else{
			if len(pidBytes)==0{
				ClearInstanceLock()
				log.Debugf("missing pid in file [%v], remove the lock file and continue",file)
				return
			}
		}
		log.Errorf("lock file:%s exists, seems instance [%v] is already running, please remove it or set `allow_multi_instance` to `true`",string(pidBytes), file)
		log.Flush()
		os.Exit(1)
	}
	FilePutContent(file, IntToString(os.Getpid()))
	log.Trace("lock placed at:", file, ", pid:", os.Getpid())
	locked = true
	p,_=filepath.Abs(p)
	log.Info("workspace: ", p)
}

// ClearInstanceLock remove the lock
func ClearInstanceLock() {
	if locked {
		err := os.Remove(path.Join(file))
		if err != nil {
			fmt.Println(err)
		}
	}
}
