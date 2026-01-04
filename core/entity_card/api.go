/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package entity_card

import (
	"context"
	"fmt"
	"sync"

	"net/http"

	//log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/security"
)

func init() {

	batchGetLabelPermission := security.GetSimplePermission("generic", "entity:label", security.Read)
	getLabelCardInfoPermission := security.GetSimplePermission("generic", "entity:card", security.Read)

	security.GetOrInitPermissionKeys(batchGetLabelPermission, getLabelCardInfoPermission)

	api.HandleUIMethod(api.POST, "/entity/label/_batch_get", batchGetLabelInfo, api.RequirePermission(batchGetLabelPermission))
	api.HandleUIMethod(api.POST, "/entity/card/:type/:id", getLabelCardInfo, api.RequirePermission(getLabelCardInfoPermission))

}

func getLabelCardInfo(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	t := ps.MustGetParameter("type")
	id := ps.MustGetParameter("id")

	provider := mustGetProviders(t)
	info := provider.GenEntityInfo(req.Context(), t, id)

	if info != nil {
		api.WriteJSON(w, info, 200)
	} else {
		api.WriteOpRecordNotFoundJSON(w, id)
	}

}

type BatchGetLabelInfoRequest struct {
	Type string   `json:"type"`
	ID   []string `json:"id"`
}

func batchGetLabelInfo(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var objs = []BatchGetLabelInfoRequest{}
	err := api.DecodeJSON(req, &objs)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	output := []EntityLabel{}
	for _, obj := range objs {
		provider := mustGetProviders(obj.Type)
		t := provider.GenEntityLabel(req.Context(), obj.Type, obj.ID)
		output = append(output, t...)
	}

	api.WriteJSON(w, output, 200)
}

type EntityProvider interface {
	GenEntityLabel(context.Context, string, []string) []EntityLabel
	GenEntityInfo(context.Context, string, string) *EntityInfo
}

func mustGetProviders(t string) EntityProvider {
	provider, ok := register.Load(t)
	if ok {
		v, ok := provider.(EntityProvider)
		if ok {
			return v
		}
	}

	panic(fmt.Sprintf("type %v not supported", t))
}

var register sync.Map

func RegisterEntityProvider(t string, provider EntityProvider) {
	register.Store(t, provider)
}
