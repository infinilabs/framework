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
	WriteJSONHeader(w)
}

// Result is a general json result
type Result struct {
	Total  int64       `json:"total"`
	Result interface{} `json:"result"`
}

// WriteJSONListResult output result list to json format
func (handler Handler) WriteJSONListResult(w http.ResponseWriter, total int64, v interface{}, statusCode int) {
	WriteJSONListResult(w, total, v, statusCode)
}

func (handler Handler) WriteError(w http.ResponseWriter, errMessage string, statusCode int) {
	WriteError(w, errMessage, statusCode)
}

func (handler Handler) WriteErrorObject(w http.ResponseWriter, err interface{}, status int) {
	WriteErrorObject(w, err, status)
}

func (handler Handler) WriteJSON(w http.ResponseWriter, v interface{}, statusCode int) {
	WriteJSON(w, v, statusCode)
}

func (handler Handler) WriteJSONBytes(w http.ResponseWriter, b []byte, statusCode int) {
	WriteJSONBytes(w, b, statusCode)
}

func (handler Handler) WriteBytes(w http.ResponseWriter, b []byte, statusCode int) {
	WriteBytes(w, b, statusCode)
}

func (handler Handler) WriteAckWithMessage(w http.ResponseWriter, ack bool, status int, msg string) {
	WriteAckWithMessage(w, ack, status, msg)
}

func NewAckJSON(ack bool) map[string]interface{} {
	v := map[string]interface{}{}
	v["acknowledged"] = ack
	return v
}

func (handler Handler) WriteAckJSON(w http.ResponseWriter, ack bool, status int, obj map[string]interface{}) {
	WriteAckJSON(w, ack, status, obj)
}

func (handler Handler) WriteAckOKJSON(w http.ResponseWriter) {
	WriteAckOKJSON(w)
}

func (handler Handler) MustGetParameter(w http.ResponseWriter, r *http.Request, key string) string {
	return MustGetParameter(w, r, key)
}

// GetParameter return query parameter with argument name
func (handler Handler) GetParameter(r *http.Request, key string) string {
	return GetParameter(r, key)
}

// GetParameterOrDefault return query parameter or return default value
func (handler Handler) GetParameterOrDefault(r *http.Request, key string, defaultValue string) string {
	return GetParameterOrDefault(r, key, defaultValue)
}

// GetIntOrDefault return parameter or default, data type is int
func (handler Handler) GetIntOrDefault(r *http.Request, key string, defaultValue int) int {
	return GetIntOrDefault(r, key, defaultValue)
}

func (handler Handler) GetBoolOrDefault(r *http.Request, key string, defaultValue bool) bool {
	return GetBoolOrDefault(r, key, defaultValue)
}

// GetJSON return json input
func (handler Handler) GetJSON(r *http.Request) (*jsonq.JsonQuery, error) {
	return GetJSON(r)
}

func (handler Handler) MustDecodeJSON(r *http.Request, o interface{}) {
	MustDecodeJSON(r, o)
}

func (handler Handler) DecodeJSON(r *http.Request, o interface{}) error {
	return DecodeJSON(r, o)
}

// GetRawBody return raw http request body
func (handler Handler) GetRawBody(r *http.Request) ([]byte, error) {
	return GetRawBody(r)
}

// Write response to client
func (handler Handler) MustWrite(w http.ResponseWriter, b []byte) {
	MustWrite(w, b)
}

func (handler Handler) Write(w http.ResponseWriter, b []byte) (int, error) {
	return Write(w, b)
}

// Error404 output 404 response
func (handler Handler) Error404(w http.ResponseWriter) {
	Error404(w)
}

// Error500 output 500 response
func (handler Handler) Error500(w http.ResponseWriter, msg string) {
	Error500(w, msg)
}

// Error output custom error
func (handler Handler) Error(w http.ResponseWriter, err error) {
	Error(w, err)
}

// Flush flush response message
func (handler Handler) Flush(w http.ResponseWriter) {
	Flush(w)
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
	WriteCreatedOKJSON(w, id)
}

func (handler Handler) WriteUpdatedOKJSON(w http.ResponseWriter, id interface{}) {
	WriteUpdatedOKJSON(w, id)
}

func (handler Handler) WriteOpRecordNotFoundJSON(w http.ResponseWriter, id interface{}) {
	WriteOpRecordNotFoundJSON(w, id)
}

func (handler Handler) WriteDeletedOKJSON(w http.ResponseWriter, id interface{}) {
	WriteDeletedOKJSON(w, id)
}

func (handler Handler) WriteGetOKJSON(w http.ResponseWriter, id, obj interface{}) {
	WriteGetOKJSON(w, id, obj)
}

func (handler Handler) WriteGetMissingJSON(w http.ResponseWriter, id string) {
	WriteGetMissingJSON(w, id)
}

func (handler Handler) Redirect(w http.ResponseWriter, r *http.Request, url string) {
	Redirect(w, r, url)
}
