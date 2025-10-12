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
	"bytes"
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
func WriteHeader(w http.ResponseWriter, code int) {

	if apiConfig != nil && apiConfig.TLSConfig.TLSEnabled {
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
	}

	// Ensure WriteHeader is only called once
	if rw, ok := w.(interface{ WriteHeader(int) }); ok {
		rw.WriteHeader(code)
	}
}

func (handler Handler) WriteHeader(w http.ResponseWriter, code int) {
	WriteHeader(w, code)
}

// Get http parameter or return default value
func (handler Handler) Get(req *http.Request, key string, defaultValue string) string {
	if !handler.formParsed {
		_ = req.ParseForm()
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
func EncodeJSON(v interface{}) (b []byte, err error) {

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
}

func (handler Handler) WriteJavascriptHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/javascript")
}

// WriteJSONHeader will write standard json header
func (handler Handler) WriteJSONHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
}

// Result is a general json result
type Result struct {
	Total  int64       `json:"total"`
	Result interface{} `json:"result"`
}

// WriteJSONListResult output result list to json format
func (handler Handler) WriteJSONListResult(w http.ResponseWriter, total int64, v interface{}, statusCode int) {
	result := Result{}
	result.Total = total
	result.Result = v
	handler.WriteJSON(w, result, statusCode)
}

func PrepareErrorJson(errMessage string, statusCode int) util.MapStr {
	err1 := util.MapStr{
		"status": statusCode,
		"error": util.MapStr{
			"reason": errMessage,
		},
	}
	return err1
}

func WriteJSON(w http.ResponseWriter, v interface{}, statusCode int) {
	WriteHeader(w, statusCode)
	_, err := w.Write(util.MustToJSONBytes(v))
	if err != nil {
		panic(err)
	}
}

func (handler Handler) WriteError(w http.ResponseWriter, errMessage string, statusCode int) {
	err1 := PrepareErrorJson(errMessage, statusCode)
	handler.WriteJSON(w, err1, statusCode)
}

func (handler Handler) WriteErrorObject(w http.ResponseWriter, err interface{}, status int) {
	if v, ok := err.(error); ok {
		handler.WriteError(w, v.Error(), status)
		return
	}
	handler.WriteError(w, util.MustToJSON(err), status)
}

func (handler Handler) WriteJSON(w http.ResponseWriter, v interface{}, statusCode int) {
	b, err := EncodeJSON(v)
	if err != nil {
		panic(err)
	}

	handler.WriteJSONBytes(w, b, statusCode)
}

func (handler Handler) WriteJSONBytes(w http.ResponseWriter, b []byte, statusCode int) {
	handler.WriteJSONHeader(w)
	handler.WriteBytes(w, b, statusCode)
}

func (handler Handler) WriteBytes(w http.ResponseWriter, b []byte, statusCode int) {
	WriteHeader(w, statusCode)
	_, err := w.Write(b)
	if err != nil {
		panic(err)
	}
}

func (handler Handler) WriteAckWithMessage(w http.ResponseWriter, ack bool, status int, msg string) {
	obj := util.MapStr{}
	obj["message"] = msg
	handler.WriteAckJSON(w, ack, status, obj)
}

func NewAckJSON(ack bool)map[string]interface{}  {
	v := map[string]interface{}{}
	v["acknowledged"] = ack
	return v
}

func (handler Handler) WriteAckJSON(w http.ResponseWriter, ack bool, status int, obj map[string]interface{}) {
	handler.WriteJSONHeader(w)
	WriteHeader(w, status)

	v := NewAckJSON(ack)

	if obj != nil {
		for k, v1 := range obj {
			v[k] = v1
		}
	}

	b, err := EncodeJSON(v)
	if err != nil {
		panic(err)
	}
	_, err = w.Write(b)
	if err != nil {
		panic(err)
	}
}

func (handler Handler) WriteAckOKJSON(w http.ResponseWriter) {
	handler.WriteAckJSON(w, true, 200, nil)
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
	_ = r.Body.Close()
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, errors.NewWithCode(err, errors.JSONIsEmpty, r.URL.String())
	}

	data := map[string]interface{}{}
	dec := json.NewDecoder(strings.NewReader(string(content)))
	err = dec.Decode(&data)
	if err != nil {
		return nil, err
	}
	jq := jsonq.NewQuery(data)

	return jq, nil
}

func (handler Handler) MustDecodeJSON(r *http.Request, o interface{}) {
	err := handler.DecodeJSON(r, o)
	if err != nil {
		panic(err)
	}
}

func (handler Handler) DecodeJSON(r *http.Request, o interface{}) error {

	content, err := ioutil.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		return err
	}
	if len(content) == 0 {
		return errors.NewWithCode(err, errors.JSONIsEmpty, r.URL.String())
	}

	return json.Unmarshal(content, o)
}

func ReadBody(r *http.Request) ([]byte, error) {
	if r.ContentLength > 0 && r.Body != nil {
		content, err := ioutil.ReadAll(r.Body)
		_ = r.Body.Close()
		if err != nil {
			return nil, err
		}
		if len(content) == 0 {
			return nil, errors.NewWithCode(err, errors.BodyEmpty, r.URL.String())
		}

		// Replace r.Body so it can be read again later
		r.Body = ioutil.NopCloser(bytes.NewBuffer(content))

		return content, nil
	}
	return nil, errors.Error("request body is nil")
}

// GetRawBody return raw http request body
func (handler Handler) GetRawBody(r *http.Request) ([]byte, error) {
	return ReadBody(r)
}

// Write response to client
func (handler Handler) MustWrite(w http.ResponseWriter, b []byte) {
	_, err := handler.Write(w, b)
	if err != nil {
		handler.Error(w, err)
	}
}

func (handler Handler) Write(w http.ResponseWriter, b []byte) (int, error) {
	handler.WriteHeader(w, 200)
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

func (handler Handler) WriteOKJSON(w http.ResponseWriter, v interface{}) {
	handler.WriteJSON(w, v, http.StatusOK)
}

func (handler Handler) Error400(w http.ResponseWriter, msg string) {
	handler.WriteError(w, msg, http.StatusBadRequest)
}

func (handler Handler) ErrorInternalServer(w http.ResponseWriter, msg string) {
	handler.WriteError(w, msg, http.StatusInternalServerError)
}

func (handler Handler) WriteCreatedOKJSON(w http.ResponseWriter, id interface{}) {
	handler.WriteJSON(w, util.MapStr{
		"_id":    id,
		"result": "created",
	}, http.StatusOK)
}

func (handler Handler) WriteUpdatedOKJSON(w http.ResponseWriter, id interface{}) {
	handler.WriteJSON(w, util.MapStr{
		"_id":    id,
		"result": "updated",
	}, http.StatusOK)
}

func (handler Handler) WriteOpRecordNotFoundJSON(w http.ResponseWriter, id interface{}) {
	handler.WriteJSON(w, util.MapStr{
		"_id":    id,
		"result": "not_found",
	}, http.StatusNotFound)
}

func (handler Handler) WriteDeletedOKJSON(w http.ResponseWriter, id interface{}) {
	handler.WriteJSON(w, util.MapStr{
		"_id":    id,
		"result": "deleted",
	}, http.StatusOK)
}

func (handler Handler) WriteGetOKJSON(w http.ResponseWriter, id, obj interface{}) {
	handler.WriteJSON(w, util.MapStr{
		"found":   true,
		"_id":     id,
		"_source": obj,
	}, 200)
}

func (handler Handler) WriteGetMissingJSON(w http.ResponseWriter, id string) {
	handler.WriteJSON(w, util.MapStr{
		"found": false,
		"_id":   id,
	}, 404)
}

func (handler Handler) Redirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusSeeOther)
}
