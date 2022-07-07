/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package api

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api/rbac"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
	"net/http"
	"os"
)

// BasicAuth register api with basic auth
func BasicAuth(h httprouter.Handle, requiredUser, requiredPassword string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Get the Basic Authentication credentials
		user, password, hasAuth := r.BasicAuth()

		if hasAuth && user == requiredUser && password == requiredPassword {
			// Delegate request to the given handle
			h(w, r, ps)
		} else {
			// Request Basic Authentication otherwise
			w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		}
	}
}

var authEnabled = false

func NeedPermission(permission string, h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if !authEnabled || CheckPermission(w, r, permission) {
			// Delegate request to the given handle
			h(w, r, ps)
		} else {
			//TODO redirect url configurable
			http.Redirect(w, r, "/auth/login/?redirect_url="+util.UrlEncode(r.URL.String()), 302)
		}
	}
}

func EnableAuth(enable bool) {
	authEnabled = enable
}

func IsAuthEnable() bool {
	return authEnabled
}

func Login(w http.ResponseWriter, r *http.Request, user, role string) {
	SetSession(w, r, "user", user)
	SetSession(w, r, "role", role)
}

func Logout(w http.ResponseWriter, r *http.Request) {
	SetSession(w, r, "user", "")
	SetSession(w, r, "role", "")
}

func GetLoginInfo(w http.ResponseWriter, r *http.Request) (user, role string) {
	ok1, u := GetSession(w, r, "user")
	ok2, v := GetSession(w, r, "role")
	if !(ok1 && ok2) {
		return "", ""
	}
	return u.(string), v.(string)
}

func CheckPermission(w http.ResponseWriter, r *http.Request, requiredPermission string) bool {
	permissions := []string{}
	permissions = append(permissions, requiredPermission)
	return CheckPermissions(w, r, permissions)
}

func CheckPermissions(w http.ResponseWriter, r *http.Request, requiredPermissions []string) bool {
	user, role := GetLoginInfo(w, r)
	log.Trace("check user, ", user, ",", role, ",", requiredPermissions)
	if user != "" && role != "" {
		//TODO remove hard-coded permission check
		if role == ROLE_ADMIN {
			return true
		}

		perms, err := GetPermissionsByRole(role)
		if err != nil {
			log.Error(err)
			return false
		}

		for _, v := range requiredPermissions {
			if v != "" && !perms.Contains(v) {
				log.Tracef("user %s with role: %s do not have permission: %s", user, role, v)
				return false
			}
		}

		log.Trace("user logged in, ", user, ",", role, ",", requiredPermissions)
		return true
	}

	log.Trace("user not logged in, ", user, ",", role, ",", requiredPermissions)
	return false
}

func (handler Handler) RequireLogin(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

		if authEnabled{
			claims, err := rbac.ValidateLogin(r.Header.Get("Authorization"))
			if err != nil {
				handler.WriteError(w, err.Error(),http.StatusUnauthorized)
				return
			}
			r = r.WithContext(rbac.NewUserContext(r.Context(), claims))
		}

		h(w, r, ps)
	}
}

func (handler Handler) RequirePermission(h httprouter.Handle, permissions ...string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

		if authEnabled {
			claims, err := rbac.ValidateLogin(r.Header.Get("Authorization"))
			if err != nil {
				handler.WriteError(w, err.Error(), http.StatusUnauthorized)
				return
			}
			err = rbac.ValidatePermission(claims, permissions)
			if err != nil {
				handler.WriteError(w, err.Error(), http.StatusForbidden)
				return
			}
			r = r.WithContext(rbac.NewUserContext(r.Context(), claims))
		}

		h(w, r, ps)
	}
}

func (handler Handler) RequireClusterPermission(h httprouter.Handle, permissions ...string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

		if authEnabled {
			id := ps.ByName("id")
			claims, err := rbac.ValidateLogin(r.Header.Get("Authorization"))
			if err != nil {
				handler.WriteError(w, err.Error(), http.StatusUnauthorized)
				return
			}
			r = r.WithContext(rbac.NewUserContext(r.Context(), claims))
			hasAllPrivilege, clusterIDs := rbac.GetCurrentUserCluster(r)
			if !hasAllPrivilege && (len(clusterIDs) == 0 || !util.StringInArray(clusterIDs, id)) {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(http.StatusText(http.StatusForbidden)))
				return
			}
		}

		h(w, r, ps)
	}
}

func (handler Handler) GetCurrentUser(req *http.Request) string {
	if authEnabled {
		claims, ok := req.Context().Value("user").(*rbac.UserClaims)
		if ok {
			return claims.Username
		}
	}
	return ""
}

const UserAdminLockFilePath = "/data/console/user_admin_lock"
func IsBuiltinUserAdminDisabled() bool{
	currentDir, _ := os.Getwd()
	targetPath := util.JoinPath(currentDir, UserAdminLockFilePath)
	return util.FileExists(targetPath)
}
func DisableBuiltinUserAdmin() error{
	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}
	targetPath := util.JoinPath(currentDir, UserAdminLockFilePath)
	if !util.FileExists(targetPath){
		_, err = util.FilePutContent(targetPath, "")
		if err != nil {
			return err
		}
	}
	return nil
}

func EnableBuiltinUserAdmin() error{
	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}
	targetPath := util.JoinPath(currentDir, UserAdminLockFilePath)
	if util.FileExists(targetPath){
		return util.FileDelete(targetPath)
	}
	return nil
}