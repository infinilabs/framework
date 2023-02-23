/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package credential

import (
	"fmt"
	"infini.sh/framework/core/orm"
)

type Credential struct {
	orm.ORMObjectBase
	Name     string `json:"name" elastic_mapping:"name:{type:keyword,copy_to:search_text}"`
	Type     string `json:"type" elastic_mapping:"type:{type:keyword}"`
	Tags []string `json:"tags" elastic_mapping:"category:{type:keyword,copy_to:search_text}"`
	Payload map[string]interface{} `json:"payload" elastic_mapping:"payload:{type:object,enabled:false}"`
	Encrypt struct{
		Type string `json:"type"`
		Params map[string]interface{} `json:"params"`
	} `json:"encrypt" elastic_mapping:"encrypt:{type:object,enabled:false}"`
	SearchText string `json:"search_text,omitempty" elastic_mapping:"search_text:{type:text,index_prefixes:{},index_phrases:true, analyzer:suggest_text_search }"`
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

func (cred *Credential) Encode() error{
	switch cred.Type {
	case BasicAuth:
		return encodeBasicAuth(cred)
	default:
		return fmt.Errorf("unkonow credential type [%s]", cred.Type)
	}
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