package badger

import log "github.com/cihub/seelog"

type BadgerSeelog struct{}

func (l *BadgerSeelog) Errorf(format string, args ...interface{}) {
	log.Errorf("[badger] "+format, args...)
}
func (l *BadgerSeelog) Warningf(format string, args ...interface{}) {
	log.Warnf("[badger] "+format, args...)
}
func (l *BadgerSeelog) Infof(format string, args ...interface{}) {
	log.Infof("[badger] "+format, args...)
}
func (l *BadgerSeelog) Debugf(format string, args ...interface{}) {
	log.Debugf("[badger] "+format, args...)
}
