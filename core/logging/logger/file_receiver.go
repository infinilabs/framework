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

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package logger

import (
	log "github.com/cihub/seelog"
	"io"
)

func NewFileReceiver(writer io.Writer, minLogLevel log.LogLevel) *FileReceiver {
	return &FileReceiver{
		writer:      writer,
		minLogLevel: minLogLevel,
	}
}

// FileReceiver is a struct of file log receiver, which implements seelog.CustomReceiver
type FileReceiver struct {
	writer      io.Writer
	minLogLevel log.LogLevel
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
