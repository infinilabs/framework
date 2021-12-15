/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package queue

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/errors"
	queue1 "infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	queue "infini.sh/framework/modules/queue/disk_queue"
	"io"
	"net/http"
	"os"
	"strings"
)

func (module *DiskQueue) RegisterAPI()  {
	api.HandleAPIMethod(api.GET,"/queue/stats", module.QueueStatsAction)
	api.HandleAPIMethod(api.GET,"/queue/:id/_scroll", module.QueueExplore)
}

func (module *DiskQueue) QueueStatsAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	include:=module.Get(req,"metadata","true")
	useKey :=module.Get(req,"use_key","true")
	datas := map[string]util.MapStr{}
	queues := queue1.GetQueues()
	for t, qs := range queues {
		data := util.MapStr{}
		for _,q:=range qs{
			qd := util.MapStr{
				"depth":module.Depth(q),
			}
			cfg,ok:=queue1.GetConfigByUUID(q)
			if include!="false" {
				if ok{
					qd["metadata"]=cfg
				}
			}

			if useKey =="false"{
				data[q]=qd
			}else{
				data[cfg.Name]=qd
			}
		}
		datas[t]=data
	}
	module.WriteJSON(w, util.MapStr{
		"queue": datas,
	}, 200)
}

func (module *DiskQueue) QueueExplore(w http.ResponseWriter, req *http.Request, ps httprouter.Params)  {

	queueName:=ps.ByName("id")
	from:= module.GetIntOrDefault(req,"from",0)
	size:= module.GetIntOrDefault(req,"size",5)
	var ctx queue.Context
	var err error
	messages:=[]util.MapStr{}
	defer func() {
		result:=util.MapStr{}
		status:=200
		if err!=nil&&strings.TrimSpace(err.Error())!=""{
			result["error"]=err
			status=500
		}
		if len(messages)>0{
			result["messages"]=messages
			result["context"]=ctx
		}
		module.WriteJSON(w,result,status)
	}()

	q,ok:=module.queues.Load(queueName)
	if ok{
		 ctx=(*q.(*queue.BackendQueue)).ReadContext()
	}else{
		err=errors.New(fmt.Sprintf("queue [%v] not exists",queueName))
		return
	}

	var msgSize int32
	readFile, err := os.OpenFile(ctx.File, os.O_RDONLY, 0600)
	defer readFile.Close()
	if err != nil {
		return
	}

	var maxBytesPerFileRead int64= module.cfg.MaxBytesPerFile
	var stat os.FileInfo
	stat, err = readFile.Stat()
	if err!=nil{
		return
	}
	maxBytesPerFileRead = stat.Size()

	var readPos int64=ctx.NextReadOffset

	if readPos > 0 {
		_, err = readFile.Seek(readPos, 0)
		if err != nil {
			return
		}
	}
	var reader= bufio.NewReader(readFile)

	var messageOffset=0
READ_MSG:

	//read message size
	err = binary.Read(reader, binary.BigEndian, &msgSize)
	if err != nil {
		return
	}

	if int(msgSize) < module.cfg.MinMsgSize || int(msgSize) > module.cfg.MaxMsgSize {
		err=errors.New("message is too big")
		return
	}

	//read message
	readBuf := make([]byte, msgSize)
	_, err = io.ReadFull(reader, readBuf)
	if err != nil {
		return
	}

	totalBytes := int64(4 + msgSize)
	nextReadPos := readPos + totalBytes
	previousPos:=readPos
	readPos=nextReadPos

	if messageOffset<from{
		messageOffset++
		goto READ_MSG
	}

	message:=util.MapStr{
		"offset":messageOffset,
		"message":string(readBuf),
		"position":previousPos,
		"next_position":nextReadPos,
	}
	messages=append(messages,message)

	if len(messages)>=size{
		return
	}

	if nextReadPos >= maxBytesPerFileRead{
		return
	}

	messageOffset++
	goto READ_MSG

}
