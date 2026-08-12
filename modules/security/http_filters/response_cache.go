/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */
package http_filters

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
	ccache "infini.sh/framework/lib/cache"
)

// maxCacheableBodyBytes caps the response size eligible for caching, so an
// opt-in route that happens to return a large payload cannot balloon memory.
const maxCacheableBodyBytes = 1 << 20 // 1 MiB

// maxRequestBodyHashBytes caps how much of a GET request body is read to mix
// into the cache key (GET-with-body is legal and can affect the response). A
// request body larger than this is served but not cached, since keying it
// reliably would mean buffering it all in memory.
const maxRequestBodyHashBytes = 1 << 20 // 1 MiB

// responseCache holds serialized GET response bodies for routes opted into
// caching via the api.Cache option. It uses the framework's standard ccache
// (LRU + TTL + size bound).
var responseCache = ccache.New(ccache.Configure().MaxSize(5000))

// isEnabled reports whether the opt-in response cache is turned on globally
// (web.response_cache.enabled). Read per-request so a config reload takes
// effect without restart. Off by default — caching only happens on routes that
// both set api.Cache and run with this flag enabled. Overridable in tests.
var isEnabled = func() bool {
	return global.Env().SystemConfig.WebAppConfig.ResponseCache.Enabled
}

// InvalidateAll purges every cached GET response. Call it when any data that
// feeds a cached route has changed and you want a blanket reset.
func InvalidateAll() {
	responseCache.Clear()
}

// InvalidatePath purges cached responses for a specific method + path (across
// all query strings, callers, and request bodies) and returns how many entries
// it removed. Call it when the data behind a particular cached route changed,
// e.g. after creating/deleting the resource that route lists.
func InvalidatePath(method, path string) int {
	return responseCache.DeletePrefix(method + ":" + path + "?")
}

// cachedResponse is what we store: the body bytes plus the Content-Type the
// handler set, so a cache hit can replay an identical response.
type cachedResponse struct {
	body        []byte
	contentType string
}

// ResponseCacheFilter serves cached GET responses for routes whose options set
// CacheTTL > 0 (via api.Cache) — and only when the web.response_cache.enabled
// feature flag is on (default off). On a miss it runs the handler, then caches
// successful (200) responses. It only applies to GET.
//
// Cache key: keyed by method + URL + the authenticated caller's user id + a
// hash of the request body. Including the caller prevents one user's response
// being served to another ("串用户"); the auth filter (priority 200) has already
// resolved the caller from Authorization/Cookie/X-API-TOKEN into the request
// context before this filter (priority 1500) runs. Hashing the body makes
// GET-with-body safe (the body is read, hashed, and restored for the handler).
//
// Priority 1500 makes it the innermost filter: auth (200), permission (500)
// and JSON masking (1000) all run first, so a cached body is only ever served
// to authorized callers and after masking is applied.
type ResponseCacheFilter struct{}

func init() {
	api.RegisterUIFilter(&ResponseCacheFilter{})
}

func (f *ResponseCacheFilter) GetPriority() int {
	// Lower values execute first.
	return 1500
}

func (f *ResponseCacheFilter) ApplyFilter(
	method string,
	pattern string,
	options *api.HandlerOptions,
	next httprouter.Handle,
) httprouter.Handle {
	if options == nil || options.CacheTTL <= 0 || method != string(api.GET) {
		return next
	}
	ttl := options.CacheTTL

	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Master feature flag (web.response_cache.enabled). Off by default;
		// when off, opted-in routes simply run the handler uncached.
		if !isEnabled() {
			next(w, r, ps)
			return
		}

		key, cacheable := cacheKeyFor(r)

		// Cache hit: replay the stored response verbatim.
		if cacheable {
			if item := responseCache.Get(key); item != nil && !item.Expired() {
				if cr, ok := item.Value().(*cachedResponse); ok {
					if cr.contentType != "" {
						w.Header().Set("Content-Type", cr.contentType)
					}
					w.Header().Set("X-API-Cache", "HIT")
					_, _ = w.Write(cr.body)
					return
				}
			}
		}

		// Miss: run the handler through a pass-through writer that also
		// buffers the body for caching.
		rec := &cacheResponseWriter{ResponseWriter: w}
		next(rec, r, ps)

		if !cacheable || rec.buf.Len() == 0 || rec.buf.Len() > maxCacheableBodyBytes {
			return
		}
		// Cache only successful, bounded responses. statusCode 0 means
		// WriteHeader was never called, i.e. the handler wrote a 200 body.
		if rec.statusCode == 0 || rec.statusCode == http.StatusOK {
			responseCache.Set(key, &cachedResponse{
				body:        rec.buf.Bytes(),
				contentType: rec.Header().Get("Content-Type"),
			}, ttl)
		}
	}
}

// cacheKeyFor builds the cache key from the URL, the authenticated caller, and
// the request body. It returns cacheable=false when the request body is too
// large to key reliably — in that case the caller should serve without caching
// rather than risk a truncated-hash collision. The request body is consumed
// and restored so the downstream handler still sees it.
func cacheKeyFor(r *http.Request) (key string, cacheable bool) {
	key = r.Method + ":" + r.URL.Path + "?" + r.URL.RawQuery + "|u=" + callerUserID(r)

	if r.Body == nil || r.ContentLength == 0 {
		return key, true
	}
	// GET-with-body: mix a hash of the body into the key. Limit the read so a
	// huge body can't be forced into memory; if it exceeds the cap, skip
	// caching rather than keying on a truncated hash.
	buf, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyHashBytes+1))
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(buf))
	if err != nil || len(buf) > maxRequestBodyHashBytes {
		return key, false
	}
	if len(buf) > 0 {
		key += "|b=" + strconv.FormatUint(uint64(util.XXHash(string(buf))), 16)
	}
	return key, true
}

// callerUserID returns the authenticated caller's user id from the request
// context, or "" when the request is unauthenticated (e.g. public routes).
func callerUserID(r *http.Request) string {
	if u, err := security.GetUserFromContext(r.Context()); err == nil && u != nil {
		return u.UserID
	}
	return ""
}

// cacheResponseWriter is a pass-through http.ResponseWriter that also buffers
// the response body so it can be cached after the handler returns. Writes still
// reach the client immediately; the buffer is best-effort.
type cacheResponseWriter struct {
	http.ResponseWriter
	statusCode int
	buf        bytes.Buffer
}

func (c *cacheResponseWriter) WriteHeader(code int) {
	c.statusCode = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *cacheResponseWriter) Write(b []byte) (int, error) {
	c.buf.Write(b) // best-effort capture; bytes.Buffer.Write never errors
	return c.ResponseWriter.Write(b)
}
