/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import (
	"testing"
)

func TestMessage(t *testing.T) {
	//readFile, err := os.OpenFile("/tmp/msg", os.O_RDONLY, 0600)
	//reader:= bufio.NewReader(readFile)
	//
	//var msgSize int32
	//var readPos int64
	//var nextReadPos int64
	////read message size
	//err = binary.Read(reader, binary.BigEndian, &msgSize)
	//if err != nil {
	//	panic(err)
	//}
	//
	////read message
	//readBuf := make([]byte, msgSize)
	//_, err = io.ReadFull(reader, readBuf)
	//if err != nil {
	//	panic(err)
	//}
	//
	////double check
	//err = binary.Read(reader, binary.BigEndian, &msgSize)
	//
	//totalBytes := int(4 + msgSize)
	//nextReadPos = readPos + int64(totalBytes)
}
