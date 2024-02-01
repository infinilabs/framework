/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package env

import "infini.sh/framework/core/config"

func  (env *Env)  GetClientConfigByEndpoint(tag,endpoint string) *config.HTTPClientConfig  {
	//TODO support client config per endpoint
	return &env.SystemConfig.HTTPClientConfig
}