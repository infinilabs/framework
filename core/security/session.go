/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
)

const UserAccessTokenSessionName = "user_session_access_token"

func init() {
	RegisterHTTPAuthFilterProvider("session_token", byAccessTokenSession, 10)
}

func byAccessTokenSession(w http.ResponseWriter, r *http.Request) (claims *UserClaims, err error) {
	exists, sessToken := api.GetSession(w, r, UserAccessTokenSessionName)
	if !exists || sessToken == nil {
		return nil, errors.Error("invalid session")
	}

	tokenStr, ok := sessToken.(string)
	if !ok {
		return nil, errors.New("authorization token is empty")
	}

	// Preallocate to avoid nil pointer during JSON unmarshal
	claims = NewUserClaims()

	token, err1 := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		secret, err := GetSecret()
		if err != nil {
			return nil, fmt.Errorf("failed to get secret key: %v", err)
		}
		return []byte(secret), nil
	})
	if err1 != nil {
		return nil, err1
	}

	if token.Valid {
		if !claims.IsValid() {
			err = errors.New("user info is not valid")
			return nil, err
		}
		if !claims.VerifyExpiresAt(time.Now(), true) {
			return nil, errors.New("token is expired")
		}
	}

	return claims, nil
}

func AddUserToSession(w http.ResponseWriter, r *http.Request, user *UserSessionInfo) (error, map[string]interface{}) {

	if user == nil {
		panic("invalid user")
	}

	// Generate access token
	token, err := GenerateJWTAccessToken(user)
	if err != nil {
		return err, nil
	}

	api.ForceSetSession(w, r, UserAccessTokenSessionName, token["access_token"], true)
	return nil, token
}

func GenerateJWTAccessToken(user *UserSessionInfo) (map[string]interface{}, error) {

	var data map[string]interface{}
	t := time.Now()
	if user.LastLogin.Timestamp == nil {
		user.LastLogin.Timestamp = &t
	}

	token1 := jwt.NewWithClaims(jwt.SigningMethodHS256, UserClaims{
		UserSessionInfo: user,
		RegisteredClaims: &jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	})

	secret, err := GetSecret()
	if err != nil {
		return nil, errors.Errorf("failed to get secret key: %v", err)
	}

	tokenString, err := token1.SignedString([]byte(secret))
	if tokenString == "" || err != nil {
		return nil, errors.Errorf("failed to generate access_token for user: %v", user)
	}

	data = util.MapStr{
		"access_token": tokenString,
		"expire_in":    time.Now().Unix() + 86400, //24h
	}

	data["status"] = "ok"

	return data, err

}
