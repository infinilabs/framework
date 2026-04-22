/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */
package security

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/golang-jwt/jwt"
	"infini.sh/framework/core/errors"
)

func byAuthorizationHeader(w http.ResponseWriter, r *http.Request) (claims *UserClaims, err error) {
	var (
		authorization = r.Header.Get("Authorization")
		ok            bool
	)

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

func ValidateLogin(w http.ResponseWriter, r *http.Request) (session *UserSessionInfo, err error) {

	var claims *UserClaims

	authHTTPFilterProvider.Range(func(key, value any) bool {
		log.Trace("checking auth filter: ", key)
		f, ok := value.(HTTPAuthFilterProvider)
		if ok {
			if claims == nil || !claims.UserSessionInfo.IsValid() {
				claims, err = f(w, r)
				if claims != nil {
					log.Debug("get valid auth info from: ", key)
					return false
				}
			}
		}
		return true
	})

	if claims == nil || !claims.UserSessionInfo.IsValid() || err != nil {
		err = errors.Errorf("invalid user info: %v", err)
		return
	}

	return claims.UserSessionInfo, nil
}

var authHTTPFilterProvider = sync.Map{}

type HTTPAuthFilterProvider func(w http.ResponseWriter, r *http.Request) (claims *UserClaims, err error)

func RegisterHTTPAuthFilterProvider(name string, f HTTPAuthFilterProvider) {
	authHTTPFilterProvider.Store(name, f)
}

func init() {
	RegisterHTTPAuthFilterProvider("bearer_token", byAuthorizationHeader)
}
