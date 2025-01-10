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

package credential

import (
	"fmt"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/orm"
)

type Credential struct {
	orm.ORMObjectBase
	Name    string                 `json:"name" elastic_mapping:"name:{type:keyword,copy_to:search_text}"`
	Type    string                 `json:"type" elastic_mapping:"type:{type:keyword}"`
	Tags    []string               `json:"tags" elastic_mapping:"category:{type:keyword,copy_to:search_text}"`
	Payload map[string]interface{} `json:"payload" elastic_mapping:"payload:{type:object,enabled:false}"`
	Encrypt struct {
		Type   string                 `json:"type"`
		Params map[string]interface{} `json:"params"`
	} `json:"encrypt" elastic_mapping:"encrypt:{type:object,enabled:false}"`
	SearchText string `json:"search_text,omitempty" elastic_mapping:"search_text:{type:text,index_prefixes:{},index_phrases:true, analyzer:suggest_text_search }"`
	secret     []byte
	Invalid    bool `json:"invalid" elastic_mapping:"invalid:{type:boolean}"`
}

func (cred *Credential) SetSecret(secret []byte) {
	cred.secret = secret
}

func (cred *Credential) Validate() error {
	if cred.Name == "" {
		return fmt.Errorf("credential name must not be empty")
	}
	if cred.Type == "" {
		return fmt.Errorf("credential type must not be empty")
	}
	if _, ok := cred.Payload[cred.Type]; !ok {
		return fmt.Errorf("credential payload with type [%s] must not be empty", cred.Type)
	}
	return nil
}

func (cred *Credential) Encode() error {
	switch cred.Type {
	case BasicAuth:
		return encodeBasicAuth(cred)
	default:
		return fmt.Errorf("unkonow credential type [%s]", cred.Type)
	}
}
func (cred *Credential) DecodeBasicAuth() (*model.BasicAuth, error) {
	var dv interface{}
	dv, err := cred.Decode()
	if err != nil {
		panic(err)
	}

	if auth, ok := dv.(model.BasicAuth); ok {
		return &auth, nil
	}
	return nil, fmt.Errorf("unkonow credential type [%s]", cred.Type)
}

func (cred *Credential) Decode() (interface{}, error) {
	switch cred.Type {
	case BasicAuth:
		return decodeBasicAuth(cred)
	default:
		return nil, fmt.Errorf("unkonow credential type [%s]", cred.Type)
	}
}

const (
	BasicAuth string = "basic_auth"
)
