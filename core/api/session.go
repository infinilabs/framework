// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"net/http"
	"os"
	"strings"
	"sync"

	log "github.com/cihub/seelog"
	"github.com/gorilla/sessions"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/util"
)

var (
	sessionName string
	sessionOnce sync.Once
)

func getSessionName() string {
	sessionOnce.Do(func() {
		id := global.Env().SystemConfig.NodeConfig.ID
		if id == "" {
			panic("missing node ID")
		}
		sessionName = "INFINI-SESSION-" + id
	})
	return sessionName
}

func GetSessionStore(r *http.Request, key string) (*sessions.Session, error) {
	return getStore().Get(r, key)
}

func GetSession(w http.ResponseWriter, r *http.Request, key string) (bool, interface{}) {
	s := getStore()
	session, err := s.Get(r, getSessionName())

	if err != nil {
		if global.Env().IsDebug {
			log.Error("Session error: ", err)
		}

		// Handle corrupted session by creating a new one
		if strings.Contains(err.Error(), "the value is not valid") {
			log.Warnf("Session corrupted, creating new session: %v", err)

			// Clear the invalid cookie
			if cookie, err := r.Cookie(getSessionName()); err == nil {
				http.SetCookie(w, &http.Cookie{
					Name:   cookie.Name,
					Value:  "",
					Path:   "/",
					MaxAge: -1,
				})
			}

			// Create fresh session
			session, err = s.New(r, getSessionName())
			if err != nil {
				log.Warnf("Failed to create new session: %v", err)
				return false, nil
			}

			// Save the new empty session immediately
			if err := session.Save(r, w); err != nil {
				log.Warnf("Failed to save new session: %v", err)
			}
		}
		return false, nil
	}

	v := session.Values[key]
	return v != nil, v
}

func SetSession(w http.ResponseWriter, r *http.Request, key string, value interface{}) bool {
	return ForceSetSession(w, r, key, value, false)
}

func ForceSetSession(w http.ResponseWriter, r *http.Request, key string, value interface{}, force bool) bool {
	s := getStore()
	var (
		session *sessions.Session
		err     error
	)
	if !force {
		session, err = s.Get(r, getSessionName())

		if err != nil {
			if strings.Contains(err.Error(), "the value is not valid") {
				log.Warnf("Session corrupted in SetSession, creating new one: %v", err)
				session, err = s.New(r, getSessionName())
				if err != nil {
					log.Warnf("Failed to create new session in SetSession: %v", err)
					return false
				}
			} else {
				log.Warnf("Failed to get session in SetSession: %v", err)
				return false
			}
		}
	} else {
		// Destroy the corrupted session completely
		if cookie, err := r.Cookie(getSessionName()); err == nil {
			http.SetCookie(w, &http.Cookie{
				Name:   cookie.Name,
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
		}
		session, err = s.New(r, getSessionName())
		if err != nil {
			log.Warnf("Failed to create new session in ForceSetSession: %v", err)
			return false
		}
	}

	session.Values[key] = value

	if err := session.Save(r, w); err != nil {
		log.Warnf("Failed to save session: %v", err)
		return false
	}

	if global.Env().IsDebug {
		log.Debugf("Session saved successfully for key: %s", key)
	}

	return true
}

func ValidateAndRecoverSession(w http.ResponseWriter, r *http.Request) (*sessions.Session, error) {
	s := getStore()
	session, err := s.Get(r, getSessionName())

	if err != nil && strings.Contains(err.Error(), "the value is not valid") {
		log.Warnf("Recovering corrupted session: %v", err)

		// Destroy the corrupted session completely
		if cookie, err := r.Cookie(getSessionName()); err == nil {
			http.SetCookie(w, &http.Cookie{
				Name:   cookie.Name,
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
		}

		// Create brand new session
		session, err = s.New(r, getSessionName())
		if err != nil {
			return nil, err
		}
	}

	return session, err
}

func DelSession(w http.ResponseWriter, r *http.Request, key string) bool {
	s := getStore()
	session, err := s.Get(r, getSessionName())
	if err != nil {
		log.Warn(err)
		return false
	}
	delete(session.Values, key)
	err = session.Save(r, w)
	if err != nil {
		log.Warn(err)
		return false
	}
	return true
}

// DestroySession remove session by creating a new empty session
func DestroySession(w http.ResponseWriter, r *http.Request) bool {

	s := getStore()
	session, err := s.New(r, getSessionName())
	if err != nil {
		if global.Env().IsDebug {
			log.Error(err)
		}
		return false
	}
	session.Options.MaxAge = -1
	err = session.Save(r, w)
	if err != nil {
		log.Warn(err)
	}
	return true
}

var store sessions.Store
var lock sync.Mutex

func getStore() sessions.Store {
	lock.Lock()
	defer lock.Unlock()

	if store != nil {
		return store
	}

	bucketKey := "cookiestore_" + global.Env().SystemConfig.NodeConfig.ID

	cookieCfg := global.Env().SystemConfig.WebAppConfig.Cookie
	if cookieCfg.AuthSecret == "" {
		key := []byte("AuthSecret")
		if ok, _ := kv.ExistsKey(bucketKey, key); ok {
			v, er := kv.GetValue(bucketKey, key)
			if er == nil && len(v) > 0 {
				cookieCfg.AuthSecret = string(v)
			}
		}
		if cookieCfg.AuthSecret == "" {
			cookieCfg.AuthSecret = util.GenerateRandomString(32)
			err := kv.AddValue(bucketKey, key, []byte(cookieCfg.AuthSecret))
			if err != nil {
				log.Error(err)
			}
		}
	}

	if cookieCfg.EncryptSecret == "" {
		key := []byte("EncryptSecret")
		if ok, _ := kv.ExistsKey(bucketKey, key); ok {
			v, er := kv.GetValue(bucketKey, key)
			if er == nil && len(v) > 0 {
				cookieCfg.EncryptSecret = string(v)
			}
		}
		if cookieCfg.EncryptSecret == "" {
			cookieCfg.EncryptSecret = util.GenerateRandomString(32)
			err := kv.AddValue(bucketKey, key, []byte(cookieCfg.EncryptSecret))
			if err != nil {
				log.Error(err)
			}
		}
	}

	if cookieCfg.MaxAge < 0 {
		cookieCfg.MaxAge = 86400 * 1
	}

	if cookieCfg.Path == "" {
		cookieCfg.Path = "/"
	}

	if cookieCfg.Store == "" {
		cookieCfg.Store = "cookie"
	}

	if cookieCfg.Store == "cookie" { //cookie
		s1 := sessions.NewCookieStore([]byte(cookieCfg.AuthSecret), []byte(cookieCfg.EncryptSecret))
		s1.Options = &sessions.Options{
			Path:     cookieCfg.Path,
			MaxAge:   cookieCfg.MaxAge,
			Domain:   cookieCfg.Domain,
			Secure:   cookieCfg.Secure, // Make sure this is set appropriately
			HttpOnly: true,             // Recommended for security
			//SameSite: http.SameSiteLaxMode, // Add SameSite policy
		}
		store = s1

	} else { //filesystem

		if cookieCfg.StorePath == "" {
			cookieCfg.StorePath = util.JoinPath(os.TempDir(), "session", util.GetUUID())
			log.Trace("session store path is empty, using temp dir: ", cookieCfg.StorePath)
			os.MkdirAll(cookieCfg.StorePath, os.ModePerm)
		}

		s1 := sessions.NewFilesystemStore(cookieCfg.StorePath, []byte(cookieCfg.AuthSecret), []byte(cookieCfg.EncryptSecret))
		s1.Options = &sessions.Options{
			Path:     cookieCfg.Path,
			MaxAge:   cookieCfg.MaxAge,
			Domain:   cookieCfg.Domain,
			Secure:   cookieCfg.Secure,
			HttpOnly: true,
			//SameSite: http.SameSiteLaxMode,
		}
		store = s1
	}

	return store

}
