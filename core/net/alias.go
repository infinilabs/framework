package net

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
)

func checkPermission() {
	log.Debug("to continue use net alias, you need to run as root user.")
	if !util.HasSudoPermission() {
		panic(errors.New("root permission are required."))
	}
}
