/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gookit/validate"
	"github.com/jmoiron/jsonq"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
)

func WriteAckWithMessage(w http.ResponseWriter, ack bool, status int, msg string) {
	obj := util.MapStr{}
	obj["message"] = msg
	WriteAckJSON(w, ack, status, obj)
}

func WriteAuthRequiredError(w http.ResponseWriter, errMessage string) {
	WriteError(w, errMessage, 401)
}

func WriteAccessDeniedError(w http.ResponseWriter, errMessage string) {
	WriteError(w, errMessage, 403)
}

func WriteInvalidRequestError(w http.ResponseWriter, errMessage string) {
	WriteError(w, errMessage, 400)
}

func WriteError(w http.ResponseWriter, errMessage string, statusCode int) {
	err1 := PrepareErrorJson(errMessage, statusCode)
	WriteJSON(w, err1, statusCode)
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
func WriteJSONHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
}

func WriteAckJSON(w http.ResponseWriter, ack bool, status int, obj map[string]interface{}) {
	WriteJSONHeader(w)
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

func WriteAckOKJSON(w http.ResponseWriter) {
	WriteAckJSON(w, true, 200, nil)
}

func MustGetParameter(w http.ResponseWriter, r *http.Request, key string) string {
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
func GetParameter(r *http.Request, key string) string {
	if r.URL == nil {
		return ""
	}
	return r.URL.Query().Get(key)
}

// GetParameterOrDefault return query parameter or return default value
func GetParameterOrDefault(r *http.Request, key string, defaultValue string) string {
	v := r.URL.Query().Get(key)
	if len(v) > 0 {
		return v
	}
	return defaultValue
}

// GetIntOrDefault return parameter or default, data type is int
func GetIntOrDefault(r *http.Request, key string, defaultValue int) int {

	v := GetParameter(r, key)
	s, ok := util.ToInt(v)
	if ok != nil {
		return defaultValue
	}
	return s

}
func GetBoolOrDefault(r *http.Request, key string, defaultValue bool) bool {

	v := strings.ToLower(GetParameter(r, key))
	if v == "false" {
		return false
	} else if v == "true" {
		return true
	}
	return defaultValue

}

// GetJSON return json input
func GetJSON(r *http.Request) (*jsonq.JsonQuery, error) {

	content, err := ioutil.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, errors.ErrorWithCode(err, errors.JSONIsEmpty, r.URL.String())
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

// WriteJSONListResult output result list to json format
func WriteJSONListResult(w http.ResponseWriter, total int64, v interface{}, statusCode int) {
	result := Result{}
	result.Total = total
	result.Result = v
	WriteJSON(w, result, statusCode)
}

func MustDecodeJSON(r *http.Request, o interface{}) {
	err := DecodeJSON(r, o)
	if err != nil {
		panic(err)
	}
}
func WriteErrorObject(w http.ResponseWriter, err interface{}, status int) {

	if v, ok := err.(error); ok {
		WriteError(w, v.Error(), status)
		return
	}
	WriteError(w, util.MustToJSON(err), status)
}

func DecodeJSON(r *http.Request, o interface{}) error {
	content, err := ioutil.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		return err
	}
	if len(content) == 0 {
		return errors.ErrorWithCode(err, errors.JSONIsEmpty, r.URL.String())
	}

	return json.Unmarshal(content, o)
}

func WriteBytes(w http.ResponseWriter, b []byte, statusCode int) {
	WriteHeader(w, statusCode)
	_, err := w.Write(b)
	if err != nil {
		panic(err)
	}
}

func WriteJSONBytes(w http.ResponseWriter, b []byte, statusCode int) {
	WriteJSONHeader(w)
	WriteBytes(w, b, statusCode)
}

// GetRawBody return raw http request body
func GetRawBody(r *http.Request) ([]byte, error) {
	return util.ReadBody(r)
}

// Write response to client
func MustWrite(w http.ResponseWriter, b []byte) {
	_, err := Write(w, b)
	if err != nil {
		Error(w, err)
	}
}

func Write(w http.ResponseWriter, b []byte) (int, error) {
	WriteHeader(w, 200)
	return w.Write(b)
}

// Error404 output 404 response
func Error404(w http.ResponseWriter) {
	WriteError(w, "404", http.StatusNotFound)
}

// Error500 output 500 response
func Error500(w http.ResponseWriter, msg string) {
	WriteError(w, msg, http.StatusInternalServerError)
}

// Error output custom error
func Error(w http.ResponseWriter, err error) {
	WriteError(w, err.Error(), http.StatusInternalServerError)
}

// Flush flush response message
func Flush(w http.ResponseWriter) {
	flusher := w.(http.Flusher)
	flusher.Flush()
}

func WriteOKJSON(w http.ResponseWriter, v interface{}) {
	WriteJSON(w, v, http.StatusOK)
}

func Error400(w http.ResponseWriter, msg string) {
	WriteError(w, msg, http.StatusBadRequest)
}

func ErrorInternalServer(w http.ResponseWriter, msg string) {
	WriteError(w, msg, http.StatusInternalServerError)
}

func WriteCreatedOKJSON(w http.ResponseWriter, id interface{}) {
	WriteJSON(w, util.MapStr{
		"_id":    id,
		"result": "created",
	}, http.StatusOK)
}

func WriteUpdatedOKJSON(w http.ResponseWriter, id interface{}) {
	WriteJSON(w, util.MapStr{
		"_id":    id,
		"result": "updated",
	}, http.StatusOK)
}

func WriteOpRecordNotFoundJSON(w http.ResponseWriter, id interface{}) {
	WriteJSON(w, util.MapStr{
		"_id":    id,
		"result": "not_found",
	}, http.StatusNotFound)
}

func WriteDeletedOKJSON(w http.ResponseWriter, id interface{}) {
	WriteJSON(w, util.MapStr{
		"_id":    id,
		"result": "deleted",
	}, http.StatusOK)
}

func WriteGetOKJSON(w http.ResponseWriter, id, obj interface{}) {
	WriteJSON(w, util.MapStr{
		"found":   true,
		"_id":     id,
		"_source": obj,
	}, 200)
}

func WriteGetMissingJSON(w http.ResponseWriter, id string) {
	WriteJSON(w, util.MapStr{
		"found": false,
		"_id":   id,
	}, 404)
}

func Redirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusSeeOther)
}

func MustValidateInput(w http.ResponseWriter, obj interface{}) {
	v := validate.Struct(obj)
	if !v.Validate() {
		panic(v.Errors)
		return
	}
}
