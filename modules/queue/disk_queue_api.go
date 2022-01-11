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
	"fmt"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/errors"
	queue1 "infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"net/http"
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
	offsetStr:= module.GetParameterOrDefault(req,"offset","0,0")
	size:= module.GetIntOrDefault(req,"size",5)
	dataIsString:=true

	consumer:="api"

	var ctx *queue1.Context
	var err error
	var timeout bool
	messages:=[]queue1.Message{}
	defer func() {
		result:=util.MapStr{}
		status:=200
		if err!=nil{
			result["error"]=err.Error()
			status=500
		}
		if len(messages)>0{
			if dataIsString{
				msgs:=[]util.MapStr{}
				for _,v:=range messages{
					msg:=util.MapStr{}
					msg["message"]=string(v.Data)
					msg["offset"]=v.Offset
					msg["size"]=v.Size
					msgs=append(msgs,msg)
				}
				result["messages"]=msgs
			}else{
				result["messages"]=messages
			}

			if ctx!=nil{
				result["context"]=ctx
			}
			result["timeout"]=timeout
			if err!=nil{
				result["error"]=err.Error()
			}
		}
		module.WriteJSON(w,result,status)
	}()

	_,ok:=module.queues.Load(queueName)
	if ok{
		ctx,messages,timeout,err=module.Consume(queueName,consumer, offsetStr,size,0)
		if err!=nil{
			return
		}
	}else{
		err=errors.New(fmt.Sprintf("queue [%v] not exists",queueName))
		return
	}

}
