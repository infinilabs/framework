package util

import (
	"fmt"
	log "github.com/cihub/seelog"
	"os"
	"path"
	"path/filepath"
)

var locked bool
var file string

// CheckInstanceLock make sure there is not a lock placed before check, and place a lock after check
func CheckInstanceLock(p string) {
	file = path.Join(p, ".lock")
	if FileExists(file) {
		log.Errorf("lock file:%s exists, seems one instance is already running, please remove it or set `allow_multi_instance` to `true`", file)
		log.Flush()
		os.Exit(1)
	}
	FilePutContent(file, IntToString(os.Getpid()))
	log.Trace("lock placed,", file, " ,pid:", os.Getpid())
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
