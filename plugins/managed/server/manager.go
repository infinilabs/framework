/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package server

import (
	"crypto/tls"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/api/rbac/enum"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/proxy"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic"
	"infini.sh/framework/plugins/managed/common"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type APIHandler struct {
	api.Handler
}

var serverInit = sync.Once{}
var configRepo common.ConfigRepo
var instanceConfigFiles = map[string][]string{}     //map instance->config files TODO lru cache, short life instance should be removed
var instanceSecrets = map[string][]common.Secrets{} //map instance->secrets TODO lru cache, short life instance should be removed

func init() {
	handler := APIHandler{}
	api.HandleAPIMethod(api.POST, common.REGISTER_API, handler.registerInstance) //client register self to config servers
	api.HandleAPIMethod(api.POST, common.ENROLL_API, handler.enrollInstance)     //config server enroll clients
	api.HandleAPIMethod(api.POST, common.SYNC_API, handler.syncConfigs)          //client sync configs from config servers

	api.HandleAPIMethod(api.POST, "/configs/_reload", handler.refreshConfigsRepo) //client sync configs from config servers

	//instance api
	api.HandleAPIMethod(api.POST, common.GEN_INSTALL_SCRIPT_API, handler.RequireLogin(handler.generateInstallCommand))
	api.HandleAPIMethod(api.GET, common.GET_INSTALL_SCRIPT_API, handler.getInstallScript)

	api.HandleAPIMethod(api.POST, "/instance", handler.RequirePermission(handler.createInstance, enum.PermissionGatewayInstanceWrite))
	api.HandleAPIMethod(api.GET, "/instance/:instance_id", handler.RequirePermission(handler.getInstance, enum.PermissionAgentInstanceRead))
	api.HandleAPIMethod(api.PUT, "/instance/:instance_id", handler.RequirePermission(handler.updateInstance, enum.PermissionAgentInstanceWrite))
	api.HandleAPIMethod(api.DELETE, "/instance/:instance_id", handler.RequirePermission(handler.deleteInstance, enum.PermissionAgentInstanceWrite))

	api.HandleAPIMethod(api.GET, "/instance/_search", handler.RequirePermission(handler.searchInstance, enum.PermissionAgentInstanceRead))

	api.HandleAPIMethod(api.POST, "/instance/stats", handler.RequirePermission(handler.getInstanceStatus, enum.PermissionAgentInstanceRead))
	api.HandleAPIMethod(api.POST, "/instance/:instance_id/_proxy", handler.RequirePermission(handler.proxy, enum.PermissionGatewayInstanceRead))
	api.HandleAPIMethod(api.POST, "/instance/try_connect", handler.RequireLogin(handler.tryConnect))

	//get elasticsearch node logs, direct fetch or via stored logs
	api.HandleAPIMethod(api.GET,  "/elasticsearch/:id/node/:node_id/logs/_list", handler.RequirePermission(handler.getLogFilesByNode, enum.PermissionAgentInstanceRead))
	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/node/:node_id/logs/_read", handler.RequirePermission(handler.getLogFileContent, enum.PermissionAgentInstanceRead))

	api.HandleAPIMethod(api.GET, "/instance/:instance_id/_nodes", handler.RequirePermission(handler.getESNodesInfo, enum.PermissionAgentInstanceRead))

	api.HandleAPIMethod(api.POST, "/instance/:instance_id/_nodes/_refresh", handler.RequirePermission(handler.refreshESNodesInfo, enum.PermissionAgentInstanceWrite))
	api.HandleAPIMethod(api.POST, "/instance/:instance_id/node/_auth", handler.RequirePermission(handler.authESNode, enum.PermissionAgentInstanceWrite))
	api.HandleAPIMethod(api.DELETE, "/instance/:instance_id/_nodes", handler.RequirePermission(handler.deleteESNode, enum.PermissionAgentInstanceWrite))
	api.HandleAPIMethod(api.POST, "/instance/:instance_id/node/_associate", handler.RequirePermission(handler.associateESNode, enum.PermissionAgentInstanceWrite))
	api.HandleAPIMethod(api.POST, "/auto_associate", handler.RequirePermission(handler.autoAssociateESNode, enum.PermissionAgentInstanceWrite))

	//api.HandleAPIMethod(api.POST, "/host/_enroll", handler.enrollHost)
	//api.HandleAPIMethod(api.GET, "/host/:host_id/agent/info",handler.GetHostAgentInfo)
	//api.HandleAPIMethod(api.GET, "/host/:host_id/processes",handler.GetHostElasticProcess)
	//api.HandleAPIMethod(api.DELETE, "/host/:host_id",handler.deleteHost)

	//delegate api to instances
	api.HandleAPIFunc("/ws_proxy", func(w http.ResponseWriter, req *http.Request) {
		log.Debug(req.RequestURI)
		endpoint := req.URL.Query().Get("endpoint")
		path := req.URL.Query().Get("path")
		var tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
		target, err := url.Parse(endpoint)
		if err != nil {
			log.Error(err)
			return
		}
		newURL, err := url.Parse(path)
		if err != nil {
			log.Error(err)
			return
		}
		req.URL.Path = newURL.Path
		req.URL.RawPath = newURL.RawPath
		req.URL.RawQuery = ""
		req.RequestURI = req.URL.RequestURI()
		req.Header.Set("HOST", target.Host)
		req.Host = target.Host
		wsProxy := NewSingleHostReverseProxy(target)
		wsProxy.Dial = (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial
		wsProxy.TLSClientConfig = tlsConfig
		wsProxy.ServeHTTP(w, req)
	})
}

func (h APIHandler) registerInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	var obj = &model.Instance{}
	err := h.DecodeJSON(req, obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
	}

	oldInst := &model.Instance{}
	oldInst.ID = obj.ID
	exists, err := orm.Get(oldInst)
	if exists {
		errMsg := fmt.Sprintf("agent [%s] already exists", obj.ID)
		h.WriteError(w, errMsg, http.StatusInternalServerError)
		return
	}

	err = orm.Create(nil, obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Infof("register instance: %v[%v], %v", obj.Name, obj.ID, obj.Endpoint)

	h.WriteAckOKJSON(w)
}

func (h APIHandler) enrollInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

}

func (h *APIHandler) getInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	id := ps.MustGetParameter("instance_id")

	obj := model.Instance{}
	obj.ID = id

	exists, err := orm.Get(&obj)
	if !exists || err != nil {
		h.WriteJSON(w, util.MapStr{
			"_id":   id,
			"found": false,
		}, http.StatusNotFound)
		return
	}
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	h.WriteJSON(w, util.MapStr{
		"found":   true,
		"_id":     id,
		"_source": obj,
	}, 200)
}

func (h *APIHandler) createInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var obj = &model.Instance{}
	err := h.DecodeJSON(req, obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	res, err := h.doConnect(obj.Endpoint, obj.BasicAuth)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	obj.ID = res.ID
	obj.Description = res.Description
	if len(res.Tags) > 0 {
		obj.Tags = res.Tags
	}
	if res.Name != "" {
		obj.Name = res.Name
	}
	obj.Application = res.Application
	res.Network = res.Network

	exists, err := orm.Get(obj)
	if err != nil && err != elastic.ErrNotFound {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	if exists {
		h.WriteError(w, "instance already registered", http.StatusInternalServerError)
		return
	}
	err = orm.Create(nil, obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	h.WriteJSON(w, util.MapStr{
		"_id":    obj.ID,
		"result": "created",
	}, 200)

}

func (h *APIHandler) deleteInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("instance_id")

	obj := model.Instance{}
	obj.ID = id

	exists, err := orm.Get(&obj)
	if !exists || err != nil {
		h.WriteJSON(w, util.MapStr{
			"_id":    id,
			"result": "not_found",
		}, http.StatusNotFound)
		return
	}

	err = orm.Delete(nil, &obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	//if sm := state.GetStateManager(); sm != nil {
	//	sm.DeleteAgent(obj.ID)
	//}
	//queryDsl := util.MapStr{
	//	"query": util.MapStr{
	//		"term": util.MapStr{
	//			"agent_id": util.MapStr{
	//				"value": id,
	//			},
	//		},
	//	},
	//}
	//err = orm.DeleteBy(model.ESNodeInfo{}, util.MustToJSONBytes(queryDsl))
	//if err != nil {
	//	h.WriteError(w, err.Error(), http.StatusInternalServerError)
	//	log.Error("delete node info error: ", err)
	//	return
	//}
	//
	//queryDsl = util.MapStr{
	//	"query": util.MapStr{
	//		"term": util.MapStr{
	//			"metadata.labels.agent_id": util.MapStr{
	//				"value": id,
	//			},
	//		},
	//	},
	//}
	//err = orm.DeleteBy(model.Setting{}, util.MustToJSONBytes(queryDsl))
	//if err != nil {
	//	h.WriteError(w, err.Error(), http.StatusInternalServerError)
	//	log.Error("delete agent settings error: ", err)
	//	return
	//}

	h.WriteDeletedOKJSON(w, id)
}

func (h *APIHandler) updateInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("instance_id")
	obj := model.Instance{}

	obj.ID = id
	exists, err := orm.Get(&obj)
	if !exists || err != nil {
		h.WriteJSON(w, util.MapStr{
			"_id":    id,
			"result": "not_found",
		}, http.StatusNotFound)
		return
	}

	id = obj.ID
	create := obj.Created
	obj = model.Instance{}
	err = h.DecodeJSON(req, &obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	//protect
	obj.ID = id
	obj.Created = create
	err = orm.Update(nil, &obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	h.WriteJSON(w, util.MapStr{
		"_id":    obj.ID,
		"result": "updated",
	}, 200)
}

//
//func (h *APIHandler) deleteInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
//	id := ps.MustGetParameter("instance_id")
//
//	obj := model.Instance{}
//	obj.ID = id
//
//	exists, err := orm.Get(&obj)
//	if !exists || err != nil {
//		h.WriteJSON(w, util.MapStr{
//			"_id":    id,
//			"result": "not_found",
//		}, http.StatusNotFound)
//		return
//	}
//
//	//check reference
//	query := util.MapStr{
//		"size": 1,
//		"query": util.MapStr{
//			"bool": util.MapStr{
//				"must": []util.MapStr{
//					{
//						"term": util.MapStr{
//							"metadata.labels.permit_nodes.id": util.MapStr{
//								"value": id,
//							},
//						},
//					},
//					{
//						"terms": util.MapStr{
//							"metadata.type": []string{"cluster_migration", "cluster_comparison"},
//						},
//					},
//				},
//				"must_not": []util.MapStr{
//					{
//						"terms": util.MapStr{
//							"status": []string{task.StatusError, task.StatusComplete},
//						},
//					},
//				},
//			},
//		},
//	}
//	q := &orm.Query{
//		RawQuery: util.MustToJSONBytes(query),
//	}
//	err, result := orm.Search(task.Task{}, q)
//	if err != nil {
//		h.WriteError(w, err.Error(), http.StatusInternalServerError)
//		log.Error(err)
//		return
//	}
//	if len(result.Result) > 0 {
//		var taskId interface{}
//		if m, ok := result.Result[0].(map[string]interface{}); ok {
//			taskId = m["id"]
//		}
//		h.WriteError(w, fmt.Sprintf("failed to delete gateway instance [%s] since it is used by task [%v]", id, taskId), http.StatusInternalServerError)
//		return
//	}
//
//	err = orm.Delete(nil, &obj)
//	if err != nil {
//		h.WriteError(w, err.Error(), http.StatusInternalServerError)
//		log.Error(err)
//		return
//	}
//
//	h.WriteJSON(w, util.MapStr{
//		"_id":    obj.ID,
//		"result": "deleted",
//	}, 200)
//}

func (h *APIHandler) searchInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	var (
		application = h.GetParameterOrDefault(req, "application", "")
		keyword     = h.GetParameterOrDefault(req, "keyword", "")
		queryDSL    = `{"query":{"bool":{"must":[%s]}}, "size": %d, "from": %d}`
		strSize     = h.GetParameterOrDefault(req, "size", "20")
		strFrom     = h.GetParameterOrDefault(req, "from", "0")
		mustBuilder = &strings.Builder{}
	)
	if keyword != "" {
		mustBuilder.WriteString(fmt.Sprintf(`{"query_string":{"default_field":"*","query": "%s"}}`, keyword))
	}

	if application != "" {
		if mustBuilder.Len() > 0 {
			mustBuilder.WriteString(",")
		}
		mustBuilder.WriteString(fmt.Sprintf(`{"term":{"application.name":"%s"}}`, application))
	}

	size, _ := strconv.Atoi(strSize)
	if size <= 0 {
		size = 20
	}
	from, _ := strconv.Atoi(strFrom)
	if from < 0 {
		from = 0
	}

	q := orm.Query{}
	queryDSL = fmt.Sprintf(queryDSL, mustBuilder.String(), size, from)
	q.RawQuery = []byte(queryDSL)

	err, res := orm.Search(&model.Instance{}, &q)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.Write(w, res.Raw)
}

func (h *APIHandler) getInstanceStatus(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var instanceIDs = []string{}
	err := h.DecodeJSON(req, &instanceIDs)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(instanceIDs) == 0 {
		h.WriteJSON(w, util.MapStr{}, http.StatusOK)
		return
	}
	q := orm.Query{}
	queryDSL := util.MapStr{
		"query": util.MapStr{
			"terms": util.MapStr{
				"_id": instanceIDs,
			},
		},
	}
	q.RawQuery = util.MustToJSONBytes(queryDSL)

	err, res := orm.Search(&model.Instance{}, &q)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	result := util.MapStr{}
	for _, item := range res.Result {
		instance := util.MapStr(item.(map[string]interface{}))
		if err != nil {
			log.Error(err)
			continue
		}
		endpoint, _ := instance.GetValue("endpoint")

		gid, _ := instance.GetValue("id")

		req := &proxy.Request{
			Endpoint: endpoint.(string),
			Method:   http.MethodGet,
			Path:     "/stats",
		}

		username, _ := instance.GetValue("basic_auth.username")
		if username != nil && username.(string) != "" {
			password, _ := instance.GetValue("basic_auth.password")
			if password != nil && password.(string) != "" {
				req.BasicAuth = &model.BasicAuth{
					Username: username.(string),
					Password: password.(string),
				}
			}
		}

		res, err := proxy.DoProxyRequest(req)
		if err != nil {
			log.Error(err)
			result[gid.(string)] = util.MapStr{}
			continue
		}
		var resMap = util.MapStr{}
		err = util.FromJSONBytes(res.Body, &resMap)
		if err != nil {
			result[gid.(string)] = util.MapStr{}
			log.Errorf("get stats of %v error: %v", endpoint, err)
			continue
		}

		result[gid.(string)] = resMap
	}
	h.WriteJSON(w, result, http.StatusOK)
}

func (h *APIHandler) proxy(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var (
		method = h.Get(req, "method", "GET")
		path   = h.Get(req, "path", "")
	)
	instanceID := ps.MustGetParameter("instance_id")

	obj := model.Instance{}
	obj.ID = instanceID

	exists, err := orm.Get(&obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	if !exists {
		h.WriteJSON(w, util.MapStr{
			"error": "gateway instance not found",
		}, http.StatusNotFound)
		return
	}
	res, err := proxy.DoProxyRequest(&proxy.Request{
		Method:        method,
		Endpoint:      obj.Endpoint,
		Path:          path,
		Body:          req.Body,
		BasicAuth:     obj.BasicAuth,
		ContentLength: int(req.ContentLength),
	})
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	h.WriteHeader(w, res.StatusCode)
	h.Write(w, res.Body)
}

func (h *APIHandler) doConnect(endpoint string, basicAuth *model.BasicAuth) (*model.Instance, error) {
	res, err := proxy.DoProxyRequest(&proxy.Request{
		Method:    http.MethodGet,
		Endpoint:  endpoint,
		Path:      "/_info",
		BasicAuth: basicAuth,
	})
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("unknow gateway version")
	}
	b := res.Body
	gres := &model.Instance{}
	err = util.FromJSONBytes(b, gres)
	return gres, err

}

func (h *APIHandler) tryConnect(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var reqBody = struct {
		Endpoint  string `json:"endpoint"`
		BasicAuth *model.BasicAuth
	}{}
	err := h.DecodeJSON(req, &reqBody)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	connectRes, err := h.doConnect(reqBody.Endpoint, reqBody.BasicAuth)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, connectRes, http.StatusOK)
}
