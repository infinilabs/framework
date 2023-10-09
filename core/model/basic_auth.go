/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package model

type BasicAuth struct {
	Username string `json:"username,omitempty" config:"username" elastic_mapping:"username:{type:keyword}"`
	Password string `json:"-" config:"password" elastic_mapping:"password:{type:keyword}"` //password should not be logged, neither in json nor in log
}
