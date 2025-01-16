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
	"github.com/jmoiron/jsonq"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"io/ioutil"
	"net/http"
	"strings"
)

// Method is object of http method
type Method string

const (
	// GET is http get method
	GET Method = "GET"
	// POST is http post method
	POST Method = "POST"
	// PUT is http put method
	PUT Method = "PUT"
	// DELETE is http delete method
	DELETE Method = "DELETE"
	// HEAD is http head method
	HEAD Method = "HEAD"

	OPTIONS Method = "OPTIONS"
)

// String return http method as string
func (method Method) String() string {
	switch method {
	case GET:
		return "GET"
	case POST:
		return "POST"
	case PUT:
		return "PUT"
	case DELETE:
		return "DELETE"
	case HEAD:
		return "HEAD"
	}
	return "N/A"
}

// Handler is the object of http handler
type Handler struct {
	wroteHeader bool
	formParsed  bool
}

// WriteHeader write status code to http header
func (handler Handler) WriteHeader(w http.ResponseWriter, code int) {

	if apiConfig != nil && apiConfig.TLSConfig.TLSEnabled {
		w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
	}

	w.WriteHeader(code)
	handler.wroteHeader = true
}

// Get http parameter or return default value
func (handler Handler) Get(req *http.Request, key string, defaultValue string) string {
	if !handler.formParsed {
		req.ParseForm()
	}
	if len(req.Form) > 0 {
		return req.Form.Get(key)
	}
	return defaultValue
}

// GetHeader return specify http header or return default value if not set
func (handler Handler) GetHeader(req *http.Request, key string, defaultValue string) string {
	v := req.Header.Get(key)
	if strings.TrimSpace(v) == "" {
		return defaultValue
	}
	return v
}

// EncodeJSON encode the object to json string
func (handler Handler) EncodeJSON(v interface{}) (b []byte, err error) {

	//if(w.Get("pretty","false")=="true"){
	b, err = json.MarshalIndent(v, "", "  ")
	//}else{
	//	b, err = json.Marshal(v)
	//}

	if err != nil {
		return nil, err
	}
	return b, nil
}

func (handler Handler) WriteTextHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
	handler.wroteHeader = true
}

// WriteJSONHeader will write standard json header
func (handler Handler) WriteJSONHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	handler.wroteHeader = true
}

// Result is a general json result
type Result struct {
	Total  int64       `json:"total"`
	Result interface{} `json:"result"`
}

// WriteJSONListResult output result list to json format
func (handler Handler) WriteJSONListResult(w http.ResponseWriter, total int64, v interface{}, statusCode int) error {
	result := Result{}
	result.Total = total
	result.Result = v
	return handler.WriteJSON(w, result, statusCode)
}

func (handler Handler) WriteError(w http.ResponseWriter, errMessage string, statusCode int) error {
	err1 := util.MapStr{
		"status": statusCode,
		"error": util.MapStr{
			"reason": errMessage,
		},
	}
	return handler.WriteJSON(w, err1, statusCode)
}

func (handler Handler) WriteJSON(w http.ResponseWriter, v interface{}, statusCode int) error {
	b, err := handler.EncodeJSON(v)
	if err != nil {
		w.Write([]byte(err.Error()))
		return err
	}

	return handler.WriteBytes(w, b, statusCode)
}

func (handler Handler) WriteBytes(w http.ResponseWriter, b []byte, statusCode int) error {
	if !handler.wroteHeader {
		handler.WriteJSONHeader(w)
		w.WriteHeader(statusCode)
	}

	_, err := w.Write(b)
	if err != nil {
		w.Write([]byte(err.Error()))
		return err
	}

	return nil
}

func (handler Handler) WriteAckJSON(w http.ResponseWriter, ack bool, status int, obj map[string]interface{}) error {
	if !handler.wroteHeader {
		handler.WriteJSONHeader(w)
		w.WriteHeader(status)
	}

	v := map[string]interface{}{}
	v["acknowledged"] = ack

	if obj != nil {
		for k, v1 := range obj {
			v[k] = v1
		}
	}

	b, err := handler.EncodeJSON(v)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	if err != nil {
		return err
	}

	return nil
}

func (handler Handler) WriteAckOKJSON(w http.ResponseWriter) error {
	return handler.WriteAckJSON(w, true, 200, nil)
}

func (handler Handler) MustGetParameter(w http.ResponseWriter, r *http.Request, key string) string {
	if r.URL == nil {
		panic("URL is nil")
	}

	v := r.URL.Query().Get(key)

	if len(v) == 0 {
		panic("missing parameter " + key)
	}

	return v
}

// GetParameter return query parameter with argument name
func (handler Handler) GetParameter(r *http.Request, key string) string {
	if r.URL == nil {
		return ""
	}
	return r.URL.Query().Get(key)
}

// GetParameterOrDefault return query parameter or return default value
func (handler Handler) GetParameterOrDefault(r *http.Request, key string, defaultValue string) string {
	v := r.URL.Query().Get(key)
	if len(v) > 0 {
		return v
	}
	return defaultValue
}

// GetIntOrDefault return parameter or default, data type is int
func (handler Handler) GetIntOrDefault(r *http.Request, key string, defaultValue int) int {

	v := handler.GetParameter(r, key)
	s, ok := util.ToInt(v)
	if ok != nil {
		return defaultValue
	}
	return s

}
func (handler Handler) GetBoolOrDefault(r *http.Request, key string, defaultValue bool) bool {

	v := strings.ToLower(handler.GetParameter(r, key))
	if v == "false" {
		return false
	} else if v == "true" {
		return true
	}
	return defaultValue

}

// GetJSON return json input
func (handler Handler) GetJSON(r *http.Request) (*jsonq.JsonQuery, error) {

	content, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, errors.NewWithCode(err, errors.JSONIsEmpty, r.URL.String())
	}

	data := map[string]interface{}{}
	dec := json.NewDecoder(strings.NewReader(string(content)))
	dec.Decode(&data)
	jq := jsonq.NewQuery(data)

	return jq, nil
}

func (handler Handler) DecodeJSON(r *http.Request, o interface{}) error {

	content, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return err
	}
	if len(content) == 0 {
		return errors.NewWithCode(err, errors.JSONIsEmpty, r.URL.String())
	}

	return json.Unmarshal(content, o)
}

// GetRawBody return raw http request body
func (handler Handler) GetRawBody(r *http.Request) ([]byte, error) {

	content, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, errors.NewWithCode(err, errors.BodyEmpty, r.URL.String())
	}
	return content, nil
}

// Write response to client
func (handler Handler) Write(w http.ResponseWriter, b []byte) (int, error) {
	return w.Write(b)
}

// Error404 output 404 response
func (handler Handler) Error404(w http.ResponseWriter) {
	handler.WriteError(w, "404", http.StatusNotFound)
}

// Error500 output 500 response
func (handler Handler) Error500(w http.ResponseWriter, msg string) {
	handler.WriteError(w, msg, http.StatusInternalServerError)
}

// Error output custom error
func (handler Handler) Error(w http.ResponseWriter, err error) {
	handler.WriteError(w, err.Error(), http.StatusInternalServerError)
}

// Flush flush response message
func (handler Handler) Flush(w http.ResponseWriter) {
	flusher := w.(http.Flusher)
	flusher.Flush()
}

func (handler Handler) WriteOKJSON(w http.ResponseWriter, v interface{}) error {
	return handler.WriteJSON(w, v, http.StatusOK)
}

func (handler Handler) Error400(w http.ResponseWriter, msg string) {
	handler.WriteError(w, msg, http.StatusBadRequest)
	return
}

func (handler Handler) ErrorInternalServer(w http.ResponseWriter, msg string) {
	handler.WriteError(w, msg, http.StatusInternalServerError)
	return
}

func (handler Handler) WriteCreatedOKJSON(w http.ResponseWriter, id interface{}) error {
	return handler.WriteJSON(w, util.MapStr{
		"_id":    id,
		"result": "created",
	}, http.StatusOK)
}

func (handler Handler) WriteUpdatedOKJSON(w http.ResponseWriter, id interface{}) error {
	return handler.WriteJSON(w, util.MapStr{
		"_id":    id,
		"result": "updated",
	}, http.StatusOK)
}
func (handler Handler) WriteDeletedOKJSON(w http.ResponseWriter, id interface{}) error {
	return handler.WriteJSON(w, util.MapStr{
		"_id":    id,
		"result": "deleted",
	}, http.StatusOK)
}

func (handler Handler) WriteGetOKJSON(w http.ResponseWriter, id, obj interface{}) error {
	return handler.WriteJSON(w, util.MapStr{
		"found":   true,
		"_id":     id,
		"_source": obj,
	}, 200)
}

func (handler Handler) WriteGetMissingJSON(w http.ResponseWriter, id string) error {
	return handler.WriteJSON(w, util.MapStr{
		"found": false,
		"_id":   id,
	}, 404)
}

func (handler Handler) Redirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusSeeOther)
}
