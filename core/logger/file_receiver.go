/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package logger

import (
	log "github.com/cihub/seelog"
	"io"
)

func NewFileReceiver(writer io.Writer, minLogLevel log.LogLevel) *FileReceiver{
	return &FileReceiver{
		writer: writer,
		minLogLevel: minLogLevel,
	}
}

// FileReceiver is a struct of file log receiver, which implements seelog.CustomReceiver
type FileReceiver struct {
	writer io.Writer
	minLogLevel     log.LogLevel
}

// ReceiveMessage impl how to receive log message
func (ar *FileReceiver) ReceiveMessage(message string, level log.LogLevel, context log.LogContextInterface) error {
	if level < ar.minLogLevel {
		return nil
	}
	if ar.writer != nil {
		_, err := ar.writer.Write([]byte(message))
		return err
	}
	return nil
}

// AfterParse nothing to do here
func (ar *FileReceiver) AfterParse(initArgs log.CustomReceiverInitArgs) error {
	return nil
}

// Flush logs
func (ar *FileReceiver) Flush() {
}

// Close logs
func (ar *FileReceiver) Close() error {
	if ar.writer != nil {
		if wc, ok := ar.writer.(io.WriteCloser); ok {
			return wc.Close()
		}
	}
	return nil
}

