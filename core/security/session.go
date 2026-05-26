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
const UserAccessTokenTTL = 24 * time.Hour

func init() {
	RegisterHTTPAuthFilterProvider("session_token", byAccessTokenSession)
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
		return errors.NewWithHTTPCode(http.StatusUnauthorized, "invalid user"), nil
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
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(UserAccessTokenTTL)),
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
		"expire_in":    time.Now().Unix() + int64(UserAccessTokenTTL/time.Second),
	}

	data["status"] = "ok"

	return data, err

}
// DecorateSessionTokenResponse keeps framework-issued account responses directly
// consumable by existing console clients while auth flows converge on framework.
func DecorateSessionTokenResponse(token map[string]interface{}, user *UserSessionInfo) {
	if token == nil || user == nil {
		return
	}

	if expiresAt := tokenExpiresAtUnix(token["expire_in"]); expiresAt > 0 {
		token["expires_at"] = expiresAt

		remaining := expiresAt - time.Now().Unix()
		if remaining < 0 {
			remaining = 0
		}
		token["expire_in"] = remaining
	}

	token["username"] = user.Login
	token["id"] = user.UserID
	token["roles"] = append([]string(nil), user.Roles...)
	token["privilege"] = GetAllPermissionsForUser(user)
	applySessionTokenResponseDecorators(token, user)
}

func tokenExpiresAtUnix(value interface{}) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	default:
		return 0
	}
}
