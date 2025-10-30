/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"bytes"
	"infini.sh/framework/core/errors"
	"io/ioutil"
	"net/http"
)

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
