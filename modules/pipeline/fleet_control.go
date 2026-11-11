/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infinilabs.com */

package pipeline

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/util"
	configs_server "infini.sh/framework/modules/configs/server"
)

// Fleet control (#125): transient commands never ride the config channel
// (SPEC §8.5) — the control plane forwards them to each target instance
// THROUGH the agent-dialed reverse channel. Managed instances sit behind
// NAT/firewalls and are not directly reachable; the reverse websocket is
// the only way in.

// seam for tests: the reverse-channel client is a process-global that
// needs a live manager websocket — handler tests swap these out.
var (
	reverseProxyRequest = configs_server.ReverseProxyRequest
	reverseIsConnected  = configs_server.ReverseIsConnected
)

// broadcastDeploymentActionHandler — POST /pipeline/deployments/:id/_pause|_resume|_stop
// Sends the control command to every instance in the deployment's scope,
// addressed by the pipeline's task name (instances key tasks by config
// name). The optional ?generation=N rides along verbatim: instances still
// on another config generation refuse the command with 409 (#120) — the
// split-brain guard the broadcast transport was missing.
func (h *PipeModule) broadcastDeploymentActionHandler(action string) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		id := ps.MustGetParameter("id")
		octx := orm.NewContextWithParent(req.Context()).DirectAccess()

		dep := PipelineDeployment{}
		dep.ID = id
		exists, err := orm.GetV2(octx, &dep)
		if err != nil || !exists {
			h.WriteOpRecordNotFoundJSON(w, id)
			return
		}
		// resolve the pipeline NAME the instances run it under
		cfg := pipeline.PipelineConfigV2{}
		cfg.ID = dep.PipelineID
		if found, gerr := orm.GetV2(octx, &cfg); gerr != nil || !found {
			h.WriteError(w, fmt.Sprintf("pipeline %s not found", dep.PipelineID), http.StatusBadRequest)
			return
		}

		instances := h.loadRegisteredInstances(octx)
		targets := deploymentTargetIDs(&dep, instances)
		if len(targets) == 0 {
			h.WriteJSON(w, util.MapStr{"total": 0, "delivered": 0, "results": map[string]interface{}{}}, http.StatusOK)
			return
		}

		gen := req.URL.Query().Get("generation")
		results := map[string]util.MapStr{}
		delivered := 0
		for _, instID := range targets {
			results[instID] = h.controlOne(instID, cfg.Name, action, gen)
			if ok, _ := results[instID]["ok"].(bool); ok {
				delivered++
			}
		}
		h.WriteJSON(w, util.MapStr{
			"total":     len(targets),
			"delivered": delivered,
			"results":   results,
		}, http.StatusOK)
	}
}

// controlOne issues one instance control command over the reverse channel
// and shapes the per-instance outcome for the broadcast aggregate.
func (h *PipeModule) controlOne(instID, taskName, action, generation string) util.MapStr {
	if reverseIsConnected != nil && !reverseIsConnected(instID) {
		return util.MapStr{"ok": false, "error": "reverse channel not connected (instance offline or pre-reverse-channel build)"}
	}
	path := fmt.Sprintf("/pipeline/task/%s/_%s", url.PathEscape(taskName), action)
	if generation != "" {
		path += "?generation=" + url.QueryEscape(generation)
	}
	res, err := reverseProxyRequest(instID, &util.Request{Method: http.MethodPost, Path: path})
	if err != nil {
		return util.MapStr{"ok": false, "error": err.Error()}
	}
	out := util.MapStr{"ok": res.StatusCode >= 200 && res.StatusCode < 300, "status": res.StatusCode}
	if len(res.Body) > 0 {
		out["body"] = string(res.Body)
	}
	return out
}

// instanceActionHandler — POST /pipeline/instances/:id/_drain|_cordon|_uncordon
// Forwards the instance-level maintenance operations (#121) to one managed
// instance through the reverse channel. Query strings pass through (drain
// accepts ?timeout_seconds=).
func (h *PipeModule) instanceActionHandler(action string) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		id := ps.MustGetParameter("id")
		if !h.instanceExists(req, id) {
			h.WriteOpRecordNotFoundJSON(w, id)
			return
		}
		path := "/pipeline/_" + action
		if qs := req.URL.RawQuery; qs != "" {
			path += "?" + qs
		}
		res, err := h.forwardToInstance(id, &util.Request{Method: http.MethodPost, Path: path})
		h.proxyReply(w, res, err)
	}
}

// instanceProxyHandler — GET /pipeline/instances/:id/proxy?path=/pipeline/...
// Read-only passthrough to an instance's own pipeline API (task detail,
// stages, flight — single-instance debugging without shell access).
// Methods stay GET: anything mutating has a dedicated action route above.
func (h *PipeModule) instanceProxyHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	target := req.URL.Query().Get("path")
	// allowlist: pipeline-scope reads only — the proxy must not become an
	// open relay onto the instance's management plane
	if !strings.HasPrefix(target, "/pipeline/") || strings.Contains(target, "..") {
		h.Error400(w, "path must target a /pipeline/ endpoint")
		return
	}
	if !h.instanceExists(req, id) {
		h.WriteOpRecordNotFoundJSON(w, id)
		return
	}
	res, err := h.forwardToInstance(id, &util.Request{Method: http.MethodGet, Path: target})
	h.proxyReply(w, res, err)
}

// forwardToInstance runs one logical request over the reverse channel.
func (h *PipeModule) forwardToInstance(instID string, r *util.Request) (*util.Result, error) {
	if reverseIsConnected != nil && !reverseIsConnected(instID) {
		return nil, fmt.Errorf("instance %s is not connected through the reverse channel", instID)
	}
	return reverseProxyRequest(instID, r)
}

// proxyReply maps a forwarded result onto the HTTP response, passing the
// instance's status code and body through verbatim (a 409 from a
// generation-guarded instance must stay a 409 for the caller).
func (h *PipeModule) proxyReply(w http.ResponseWriter, res *util.Result, err error) {
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusBadGateway)
		return
	}
	status := res.StatusCode
	if status == 0 {
		status = http.StatusBadGateway
	}
	w.Header().Set("Content-Type", util.ContentTypeJson)
	w.WriteHeader(status)
	_, _ = w.Write(res.Body)
}

func (h *PipeModule) instanceExists(req *http.Request, id string) bool {
	octx := orm.NewContextWithParent(req.Context()).DirectAccess()
	inst := model.Instance{}
	inst.ID = id
	found, err := orm.GetV2(octx, &inst)
	return err == nil && found
}
