/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package fasthttp

import "testing"

func TestCtxEncode(t *testing.T) {
	ctx:=RequestCtx{}
	ctx.Request=Request{}
	ctx.Request.SetRequestURI("/favicon.ico")
}