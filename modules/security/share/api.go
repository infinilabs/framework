/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package share

import (
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"net/http"
)

type APIHandler struct {
	api.Handler
}

func (h APIHandler) batchGetShares(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	obj := []ResourceEntity{}
	h.MustDecodeJSON(req, &obj)

	ctx := orm.NewContextWithParent(req.Context())
	service := NewSharingService()

	docs, err := service.BatchGetShares(ctx, "", obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.WriteJSON(w, docs, 200)
}

type ShareRequest struct {
	Shares  []SharingRecord `json:"shares"`
	Revokes []SharingRecord `json:"revokes"`
}

type BulkOpResponses[T any] struct {
	Created   []*T `json:"created,omitempty"`
	Deleted   []*T `json:"deleted,omitempty"`
	Updated   []*T `json:"updated,omitempty"`
	Unchanged []*T `json:"unchanged,omitempty"`
}

// NewBulkOpResponses initializes an empty response container
func NewBulkOpResponses[T any]() *BulkOpResponses[T] {
	return &BulkOpResponses[T]{}
}
func (r *BulkOpResponses[T]) AddCreated(item *T) {
	r.Created = append(r.Created, item)
}
func (r *BulkOpResponses[T]) AddDeleted(item *T) {
	r.Deleted = append(r.Deleted, item)
}
func (r *BulkOpResponses[T]) AddUpdated(item *T) {
	r.Updated = append(r.Updated, item)
}

func (r *BulkOpResponses[T]) AddUnchanged(item *T) {
	r.Unchanged = append(r.Unchanged, item)
}

func (h APIHandler) createOrUpdateShare(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	op := ShareRequest{}
	h.MustDecodeJSON(req, &op)

	resourceType := ps.MustGetParameter("type")
	resourceID := ps.MustGetParameter("id")

	ctx := orm.NewContextWithParent(req.Context())
	ctx.Refresh = orm.WaitForRefresh

	sessionUser := security.MustGetUserFromContext(ctx.Context)
	userID := sessionUser.MustGetUserID()

	newOp := ShareRequest{}
	for _, v := range op.Shares {
		v.ResourceType = resourceType
		v.ResourceID = resourceID
		newOp.Shares = append(newOp.Shares, v)
	}
	for _, v := range op.Revokes {
		v.ResourceType = resourceType
		v.ResourceID = resourceID
		newOp.Revokes = append(newOp.Revokes, v)
	}

	service := NewSharingService()
	lists, err := service.CreateOrUpdateShares(ctx, userID, &newOp)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteOKJSON(w, lists)

}

func (h APIHandler) batchCreateOrUpdateShare(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	op := ShareRequest{}
	h.MustDecodeJSON(req, &op)

	ctx := orm.NewContextWithParent(req.Context())
	ctx.Refresh = orm.WaitForRefresh

	sessionUser := security.MustGetUserFromContext(ctx.Context)
	userID := sessionUser.MustGetUserID()

	service := NewSharingService()
	lists, err := service.CreateOrUpdateShares(ctx, userID, &op)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteOKJSON(w, lists)

}

func init() {
	orm.MustRegisterSchemaWithIndexName(&SharingRecord{}, "sharing-record")

	createSharePermission := security.GetSimplePermission("generic", "sharing", security.Create)
	updateSharePermission := security.GetSimplePermission("generic", "sharing", security.Update)

	deleteSharePermission := security.GetSimplePermission("generic", "sharing", security.Delete)
	readSharePermission := security.GetSimplePermission("generic", "sharing", security.Read)
	searchSharePermission := security.GetSimplePermission("generic", "sharing", security.Search)
	security.RegisterPermissionsToRole(security.RoleAdmin, createSharePermission, updateSharePermission, deleteSharePermission, readSharePermission, searchSharePermission)

	hander := APIHandler{}
	api.HandleUIMethod(api.POST, "/resources/:type/:id/share", hander.createOrUpdateShare, api.RequirePermission(createSharePermission))
	api.HandleUIMethod(api.POST, "/resources/shares/_batch_set", hander.batchCreateOrUpdateShare, api.RequirePermission(createSharePermission))
	api.HandleUIMethod(api.POST, "/resources/shares/_batch_get", hander.batchGetShares, api.RequirePermission(readSharePermission))
}
