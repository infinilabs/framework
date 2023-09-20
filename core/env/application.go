/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package env

type Application struct {
	Name    string  `json:"name,omitempty" elastic_mapping:"name:{type:keyword,fields:{text: {type: text}}}"`
	Version Version `json:"version,omitempty" elastic_mapping:"version: { type: object }"`
	Tagline string  `json:"tagline,omitempty" elastic_mapping:"tagline: { type: keyword }"`
}

func (env *Env) GetApplicationInfo() Application {
	return Application{
		Name:    env.name,
		Tagline: env.desc,
		Version: env.GetVersionInfo(),
	}
}
