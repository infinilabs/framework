/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */
package security

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/golang-jwt/jwt/v4"
	"infini.sh/framework/core/errors"
)

func parseUserClaimsFromAuthorizationHeader(authorization string) (claims *UserClaims, err error) {
	var ok bool

	if authorization == "" {
		return nil, errors.Error("Authorization not found")
	}

	fields := strings.Fields(authorization)
	if fields[0] != "Bearer" || len(fields) != 2 {
		err = errors.New("authorization header is invalid")
		return nil, err
	}
	tokenString := fields[1]

	token, err := jwt.ParseWithClaims(tokenString, NewUserClaims(), func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		secret, err := GetSecret()
		if err != nil {
			return nil, fmt.Errorf("failed to get secret key: %v", err)
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok = token.Claims.(*UserClaims)

	if ok && token.Valid {
		if !claims.IsValid() {
			err = errors.New("user info is not valid")
			return nil, err
		}
		if !claims.VerifyExpiresAt(time.Now(), true) {
			err = errors.New("token is expire in")
			return nil, err
		}
	}
	if claims == nil {
		return nil, errors.Error("invalid claims")
	}
	return claims, nil
}

// ValidateAuthorizationHeader validates a bearer token header and returns the
// decoded framework session information for callers that only have header access.
func ValidateAuthorizationHeader(authorization string) (*UserSessionInfo, error) {
	claims, err := parseUserClaimsFromAuthorizationHeader(authorization)
	if err != nil {
		return nil, err
	}
	return claims.UserSessionInfo, nil
}

func byAuthorizationHeader(w http.ResponseWriter, r *http.Request) (claims *UserClaims, err error) {
	return parseUserClaimsFromAuthorizationHeader(r.Header.Get("Authorization"))
}

func ValidateLogin(w http.ResponseWriter, r *http.Request) (session *UserSessionInfo, err error) {

	var claims *UserClaims

	authFilterMu.RLock()
	entries := make([]namedFilterEntry, len(authFilterProviders))
	copy(entries, authFilterProviders)
	authFilterMu.RUnlock()

	for _, entry := range entries {
		log.Trace("checking auth filter: ", entry.name)
		if claims == nil || !claims.UserSessionInfo.IsValid() {
			claims, err = entry.fn(w, r)
			if claims != nil {
				log.Debug("get valid auth info from: ", entry.name)
				break
			}
		}
	}

	if claims == nil || !claims.UserSessionInfo.IsValid() || err != nil {
		err = errors.Errorf("invalid user info: %v", err)
		return
	}

	return claims.UserSessionInfo, nil
}

type namedFilterEntry struct {
	name     string
	priority int
	fn       HTTPAuthFilterProvider
}

var (
	authFilterMu        sync.RWMutex
	authFilterProviders []namedFilterEntry
)

type HTTPAuthFilterProvider func(w http.ResponseWriter, r *http.Request) (claims *UserClaims, err error)

// RegisterHTTPAuthFilterProviderWithPriority registers an auth filter provider with an explicit priority.
// Smaller priority values execute first (higher precedence).
func RegisterHTTPAuthFilterProviderWithPriority(name string, f HTTPAuthFilterProvider, priority int) {
	authFilterMu.Lock()
	defer authFilterMu.Unlock()

	entry := namedFilterEntry{name: name, priority: priority, fn: f}
	i := sort.Search(len(authFilterProviders), func(i int) bool {
		return authFilterProviders[i].priority >= priority
	})
	authFilterProviders = append(authFilterProviders, namedFilterEntry{})
	copy(authFilterProviders[i+1:], authFilterProviders[i:])
	authFilterProviders[i] = entry
}

// RegisterHTTPAuthFilterProvider registers an auth filter provider with the default priority (100).
// It executes after any provider registered with an explicit priority lower than 100.
func RegisterHTTPAuthFilterProvider(name string, f HTTPAuthFilterProvider) {
	RegisterHTTPAuthFilterProviderWithPriority(name, f, 100)
}

func init() {
	RegisterHTTPAuthFilterProviderWithPriority("bearer_token", byAuthorizationHeader, 20)
}
