/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	log "github.com/cihub/seelog"
	"github.com/gorilla/sessions"
	"infini.sh/framework/core/global"
	"net/http"
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
		log.Error(err)
		return false
	}
	session.Values[key] = value
	err = session.Save(r, w)
	if err != nil {
		log.Error(err)
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

// GetFlash get flash value
func GetFlash(w http.ResponseWriter, r *http.Request) (bool, []interface{}) {
	log.Trace("get flash")
	session, err := getStore().Get(r, sessionName)
	if err != nil {
		log.Error(err)
		return false, nil
	}
	f := session.Flashes()
	log.Trace(f)
	return f != nil, f
}

// SetFlash set flash value
func SetFlash(w http.ResponseWriter, r *http.Request, msg string) bool {
	log.Trace("set flash")
	session, err := getStore().Get(r, sessionName)
	if err != nil {
		log.Error(err)
		return false
	}
	session.AddFlash(msg)
	session.Save(r, w)
	return true
}

var store *sessions.CookieStore
var lock sync.Mutex

func getStore() *sessions.CookieStore {
	lock.Lock()
	defer lock.Unlock()

	if store != nil {
		return store
	}

	cookieCfg := global.Env().SystemConfig.Cookie
	if cookieCfg.Secret == "" {
		log.Trace("use default cookie secret")
		store = sessions.NewCookieStore([]byte("APP-SECRET"))
	} else {
		log.Trace("get cookie secret from config,", cookieCfg.Secret)
		store = sessions.NewCookieStore([]byte(cookieCfg.Secret))
	}

	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 1,
		HttpOnly: true,
		Domain: cookieCfg.Domain,
	}

	return store

}
