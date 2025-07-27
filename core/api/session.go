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
	log "github.com/cihub/seelog"
	"github.com/gorilla/sessions"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"net/http"
	"os"
	"sync"
)

const sessionName string = "INFINI-SESSION"

func GetSessionStore(r *http.Request, key string) (*sessions.Session, error) {
	return getStore().Get(r, key)
}

// GetSession return session by session key
func GetSession(r *http.Request, key string) (bool, interface{}) {
	s := getStore()
	session, err := s.Get(r, sessionName)
	if err != nil {
		log.Error(err)
		return false, nil
	}

	v := session.Values[key]
	return v != nil, v
}

// SetSession set session by session key and session value
func SetSession(w http.ResponseWriter, r *http.Request, key string, value interface{}) bool {
	s := getStore()
	session, err := s.Get(r, sessionName)
	if err != nil {
		log.Warnf("Session corrupted or missing, creating new one: %v", err)
		session, _ = s.New(r, sessionName) // always safe to create new
	}

	session.Values[key] = value
	if err := session.Save(r, w); err != nil {
		log.Errorf("Failed to save session: %v", err)
		return false
	}

	if global.Env().IsDebug {
		log.Infof("Set-Cookie: %v", w.Header().Get("Set-Cookie"))
	}

	return true
}

func DelSession(w http.ResponseWriter, r *http.Request, key string) bool {
	s := getStore()
	session, err := s.Get(r, sessionName)
	if err != nil {
		log.Error(err)
		return false
	}
	delete(session.Values, key)
	err = session.Save(r, w)
	if err != nil {
		log.Error(err)
		return false
	}
	return true
}

// DestroySession remove session by creating a new empty session
func DestroySession(w http.ResponseWriter, r *http.Request) bool {
	s := getStore()
	session, err := s.New(r, sessionName)
	if err != nil {
		log.Error(err)
		return false
	}
	session.Options.MaxAge = -1
	err = session.Save(r, w)
	if err != nil {
		log.Error(err)
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

	cookieCfg := global.Env().SystemConfig.Cookie
	if cookieCfg.AuthSecret == "" {
		cookieCfg.AuthSecret = util.GenerateRandomString(32)
	}
	if cookieCfg.EncryptSecret == "" {
		cookieCfg.EncryptSecret = util.GenerateRandomString(32)
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
			HttpOnly: true,
			Secure:   false, // must be false for HTTP
			Domain:   cookieCfg.Domain,
			SameSite: http.SameSiteLaxMode,
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
			HttpOnly: true,
			//Secure:   false, // must be false for HTTP
			//Domain:   cookieCfg.Domain,
			//SameSite: http.SameSiteLaxMode,
		}
		store = s1
	}

	return store

}
