// Copyright 2020 The Go-Guardian. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/shaj13/libcache"
	_ "github.com/shaj13/libcache/fifo"

	"github.com/rubyniu105/framework/lib/guardian/auth"
	"github.com/rubyniu105/framework/lib/guardian/auth/strategies/kubernetes"
)

// Usage:
// Run kubernetes mock api and get agent token
// go run mock.go
// Request server to verify token and get book author
// curl  -k http://127.0.0.1:8080/v1/book/1449311601 -H "Authorization: Bearer  <agent-token-from-mock>"

var strategy auth.Strategy
var cacheObj libcache.Cache

func main() {
	setupGoGuardian()
	router := mux.NewRouter()

	router.HandleFunc("/v1/book/{id}", middleware(http.HandlerFunc(getBookAuthor))).Methods("GET")
	log.Println("server started and listening on http://127.0.0.1:8080")
	http.ListenAndServe("127.0.0.1:8080", router)
}

func getBookAuthor(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	books := map[string]string{
		"1449311601": "Ryan Boyd",
		"148425094X": "Yvonne Wilson",
		"1484220498": "Prabath Siriwarden",
	}
	body := fmt.Sprintf("Author: %s \n", books[id])
	w.Write([]byte(body))
}

func setupGoGuardian() {
	cacheObj = libcache.FIFO.New(0)
	cacheObj.SetTTL(time.Minute * 5)
	cacheObj.RegisterOnExpired(func(key, _ interface{}) {
		cacheObj.Peek(key)
	})
	strategy = kubernetes.New(cacheObj)
}

func middleware(next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("Executing Auth Middleware")
		user, err := strategy.Authenticate(r.Context(), r)
		if err != nil {
			code := http.StatusUnauthorized
			http.Error(w, http.StatusText(code), code)
			return
		}
		log.Printf("User %s Authenticated\n", user.GetUserName())
		next.ServeHTTP(w, r)
	})
}
