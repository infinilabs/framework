/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package model

import (
	"infini.sh/framework/lib/go-ucfg"
)

type BasicAuth struct {
	Username string `json:"username,omitempty" config:"username" elastic_mapping:"username:{type:keyword}"`
	//password should not be logged, neither in json nor in log
	Password ucfg.SecretString `json:"password,omitempty" config:"password" yaml:"password" elastic_mapping:"password:{type:keyword}"`
}