/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"context"
	"net/http"
	"sort"
	"sync"
)

// RequestContextResolver enriches the user session after authentication.
// Implementations can read request headers (e.g. X-Team-ID, X-Project-ID)
// and populate the session Parameters accordingly.
type RequestContextResolver interface {
	ResolveRequestContext(ctx context.Context, req *http.Request, session *UserSessionInfo) error
	Priority() int
}

var (
	resolversMu      sync.RWMutex
	requestResolvers []RequestContextResolver
	resolversSorted  bool
)

// RegisterRequestContextResolver adds a resolver to the chain.
func RegisterRequestContextResolver(r RequestContextResolver) {
	resolversMu.Lock()
	defer resolversMu.Unlock()
	requestResolvers = append(requestResolvers, r)
	resolversSorted = false
}

// RunRequestContextResolvers invokes all registered resolvers in priority order.
// Called after successful authentication to populate team/project context.
func RunRequestContextResolvers(ctx context.Context, req *http.Request, session *UserSessionInfo) error {
	resolversMu.RLock()
	if !resolversSorted {
		resolversMu.RUnlock()
		resolversMu.Lock()
		if !resolversSorted {
			sort.Slice(requestResolvers, func(i, j int) bool {
				return requestResolvers[i].Priority() < requestResolvers[j].Priority()
			})
			resolversSorted = true
		}
		resolversMu.Unlock()
		resolversMu.RLock()
	}
	resolvers := requestResolvers
	resolversMu.RUnlock()

	for _, r := range resolvers {
		if err := r.ResolveRequestContext(ctx, req, session); err != nil {
			return err
		}
	}
	return nil
}
