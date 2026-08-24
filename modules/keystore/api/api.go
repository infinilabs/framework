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

package api

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/keystore"
	"infini.sh/framework/core/util"
	kslib "infini.sh/framework/lib/keystore"
	"net/http"
	"sort"
)

type APIHandler struct {
	api.Handler
}

func (h *APIHandler) setKeystoreValue(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	reqBody := struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}{}
	err := h.DecodeJSON(req, &reqBody)
	if err != nil {
		_ = log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if reqBody.Key == "" {
		h.WriteError(w, "key cannot be empty", http.StatusInternalServerError)
		return
	}
	ks, err := keystore.GetWriteableKeystore()
	if err != nil {
		_ = log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = ks.Store(reqBody.Key, util.UnsafeStringToBytes(reqBody.Value))
	if err != nil {
		_ = log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = ks.Save()
	if err != nil {
		_ = log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, util.MapStr{
		"success": true,
	}, http.StatusOK)
}


// listKeystoreKeys — GET /keystore — key names only. Values are secrets:
// the API is write-only for them by design (nothing can read them back).
func (h *APIHandler) listKeystoreKeys(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	ks, err := keystore.GetOrInitKeystore()
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	listKs, err := kslib.AsListingKeystore(ks)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	keys, err := listKs.List()
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Strings(keys)
	h.WriteJSON(w, util.MapStr{"keys": keys}, http.StatusOK)
}

// deleteKeystoreKey — DELETE /keystore?key=...
func (h *APIHandler) deleteKeystoreKey(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	key := req.URL.Query().Get("key")
	if key == "" {
		h.WriteError(w, "key cannot be empty", http.StatusBadRequest)
		return
	}
	ks, err := keystore.GetWriteableKeystore()
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := ks.Delete(key); err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := ks.Save(); err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, util.MapStr{"deleted": true, "key": key}, http.StatusOK)
}
