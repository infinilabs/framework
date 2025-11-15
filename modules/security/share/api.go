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

// Return all current shares (users/groups/links) for a resource.
func (h APIHandler) listShares(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	ctx := orm.NewContextWithParent(req.Context())
	service := NewSharingService()
	docs, err := service.ListShares(ctx, req)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.WriteJSON(w, docs, 200)
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

//// Create a Share Link
////
////	req: {
////	 "permission": "view", // or "edit"
////	 "expires_at": "2025-12-01T00:00:00Z"
////	}
////
//// res:
////
////	{
////	 "url": "https://coco.rs/s/abc123",
////	 "token": "abc123",
////	 "permission": "view"
////	}
//func (h APIHandler) createShareLink(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
//	var request struct {
//		ResourcePath string     `json:"resource_path"`
//		Permission   string     `json:"permission"`
//		ExpiresAt    *time.Time `json:"expires_at,omitempty"`
//		Password     string     `json:"password,omitempty"`
//	}
//
//	h.MustDecodeJSON(req, &request)
//
//	resourceType := ps.MustGetParameter("type")
//	resourceID := ps.MustGetParameter("id")
//
//	ctx := orm.NewContextWithParent(req.Context())
//	sessionUser := security.MustGetUserFromContext(ctx.Context)
//	userID:=sessionUser.MustGetUserID()
//
//	service := NewSharingService()
//	link, err := service.CreateShareLink(ctx, resourceType, resourceID, request.ResourcePath, userID, request.Permission, request.ExpiresAt, request.Password)
//	if err != nil {
//		h.WriteError(w, err.Error(), http.StatusInternalServerError)
//		return
//	}
//
//	response := map[string]interface{}{
//		"url":        fmt.Sprintf("/s/%s", link.Token),
//		"token":      link.Token,
//		"permission": permissionToString(link.Permission),
//		"expires_at": link.ExpiresAt,
//	}
//
//	h.WriteJSON(w, response, http.StatusCreated)
//}

// Share Resource
// req:
//
//	{
//	 "shares": [
//	   {
//	     "principal_id": "usr_002",
//	     "principal_type": "user",
//	     "permission": "edit"
//	   },
//	   {
//	     "principal_id": "grp_marketing",
//	     "principal_type": "group",
//	     "permission": "view"
//	   }
//	 ]
//	}
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

// Remove Share
func (h APIHandler) removeAllShares(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resourceID := ps.MustGetParameter("id")
	shareID := ps.MustGetParameter("share_id")

	ctx := orm.NewContextWithParent(req.Context())
	sessionUser := security.MustGetUserFromContext(ctx.Context)
	userID := sessionUser.MustGetUserID()

	service := NewSharingService()
	err := service.RemoveShare(ctx, resourceID, shareID, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "share not found" {
			status = http.StatusNotFound
		} else if err.Error() == "unauthorized: you can only remove shares you created" {
			status = http.StatusForbidden
		}
		h.WriteError(w, err.Error(), status)
		return
	}

	h.WriteDeletedOKJSON(w, shareID)
}

// Returns the current user's effective access level.
//
//	{
//	 "resource_type": "google_drive_001",
//	 "resource_id": "res_12345",
//	 "permission": "edit",
//	 "via": "direct" // or "group", "link"
//	}
func (h APIHandler) getMyAccessForResource(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resourceType := ps.MustGetParameter("type")
	resourceID := ps.MustGetParameter("id")
	path := h.GetParameter(req, "resource_path")

	ctx := orm.NewContextWithParent(req.Context())
	sessionUser := security.MustGetUserFromContext(ctx.Context)
	userID := sessionUser.MustGetUserID()

	service := NewSharingService()
	response, err := service.GetMyAccessForResource(ctx, userID, resourceType, resourceID, path)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.WriteJSON(w, response, http.StatusOK)
}

func init() {
	orm.MustRegisterSchemaWithIndexName(&SharingRecord{}, "sharing-record")
	orm.MustRegisterSchemaWithIndexName(&ShareLink{}, "sharing-links")

	createSharePermission := security.GetSimplePermission("generic", "sharing", security.Create)
	updateSharePermission := security.GetSimplePermission("generic", "sharing", security.Update)

	deleteSharePermission := security.GetSimplePermission("generic", "sharing", security.Delete)
	readSharePermission := security.GetSimplePermission("generic", "sharing", security.Read)
	searchSharePermission := security.GetSimplePermission("generic", "sharing", security.Search)
	security.RegisterPermissionsToRole(security.RoleAdmin, createSharePermission, updateSharePermission, deleteSharePermission, readSharePermission, searchSharePermission)

	hander := APIHandler{}
	//api.HandleUIMethod(api.GET, "/resources/:type/:id/access", hander.getMyAccessForResource, api.RequirePermission(readSharePermission))
	//api.HandleUIMethod(api.GET, "/resources/:type/:id/shares", hander.listShares, api.RequirePermission(readSharePermission))
	//api.HandleUIMethod(api.POST, "/resources/:type/:id/share_links", hander.createShareLink, api.RequirePermission(createSharePermission))
	api.HandleUIMethod(api.POST, "/resources/:type/:id/share", hander.createOrUpdateShare, api.RequirePermission(createSharePermission))
	api.HandleUIMethod(api.POST, "/resources/shares/_batch_set", hander.batchCreateOrUpdateShare, api.RequirePermission(createSharePermission))
	api.HandleUIMethod(api.POST, "/resources/shares/_batch_get", hander.batchGetShares, api.RequirePermission(readSharePermission))
	//api.HandleUIMethod(api.DELETE, "/resources/:type/:id/share/:share_id", hander.removeAllShares, api.RequirePermission(deleteSharePermission))
}
