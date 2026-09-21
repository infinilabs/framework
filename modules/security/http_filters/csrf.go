/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package http_filters

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/security"
)

func init() {
	api.RegisterUIFilter(&CSRFFilter{})
}

// CSRF 防护（会话绑定 token，无服务端存储）：
//
// token = hex(HMAC-SHA256(framework_secret, session_jwt))——随会话稳定（多标签页
// 无竞态）、重启不变（secret 持久化于 KV）。跨站攻击者既读不到会话 JWT，也无法
// 跨域携带自定义 X-CSRFToken 头，因此 token 不落 cookie 即可完成校验（等效于
// double-submit，且无子域 cookie 注入面）。
//
// 用法：路由注册时加 api.Feature(api.FeatureCSRF) 即启用（opt-in，不影响既有应用）。
// token 下发：GET 端点或 bootstrap 里调用 CSRFTokenForSession 返回给前端；前端对
// 写请求带 X-CSRFToken 头。sendBeacon 等无法携带自定义头的端点不要开启本特性。
const CSRFHeader = "X-CSRFToken"

// CSRFTokenForSession derives the CSRF token bound to the current session.
// Returns an empty string when there is no valid session (callers that only
// serve the token to logged-in users should treat that as "no token").
func CSRFTokenForSession(w http.ResponseWriter, r *http.Request) string {
	secret, err := security.GetSecret()
	if err != nil {
		return ""
	}
	exists, sessToken := api.GetSession(w, r, security.UserAccessTokenSessionName)
	if !exists || sessToken == nil {
		return ""
	}
	jwtStr, ok := sessToken.(string)
	if !ok || jwtStr == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(jwtStr))
	return hex.EncodeToString(mac.Sum(nil))
}

type CSRFFilter struct {
	api.Handler
}

func (f *CSRFFilter) GetPriority() int {
	// 在 AuthFilter(200) 之后、PermissionFilter(500) 之前执行
	return 300
}

func (f *CSRFFilter) ApplyFilter(
	method string,
	pattern string,
	options *api.HandlerOptions,
	next httprouter.Handle,
) httprouter.Handle {
	if options == nil || !options.Feature(api.FeatureCSRF) {
		return next
	}
	switch api.Method(method) {
	case api.POST, api.PUT, api.DELETE:
	default:
		return next
	}

	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		expected := CSRFTokenForSession(w, r)
		got := r.Header.Get(CSRFHeader)
		if expected == "" || !hmac.Equal([]byte(expected), []byte(got)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"CSRF token missing or invalid"}`))
			return
		}
		next(w, r, ps)
	}
}
