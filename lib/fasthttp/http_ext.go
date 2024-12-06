/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package fasthttp

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/util"
)

const HeaderContentEncoding2 = "Content-Encoding"

//func (req *Request) IsGzipped() bool {
//	ce := string(req.Header.PeekAny([]string{HeaderContentEncoding, HeaderContentEncoding2}))
//	if ce == "gzip" {
//		return true
//	}
//	return false
//}

func (h *RequestHeader) PeekAny(keys []string) []byte {
	for _, k := range keys {
		v := h.Peek(k)
		if len(v) > 0 {
			return v
		}
	}
	return nil
}

func GetBasicAuthHeader(user string, password string) string {
	var userAndPass = user + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(userAndPass))
}

func (req *Request) SetBasicAuth(username string, password string) {
	req.Header.Del("Authorization")
	req.Header.Set("Authorization", GetBasicAuthHeader(username, password))
	req.getURI().ResetUser()
}

//func (req *Request) GetRawBody() []byte {
//	ce := string(req.Header.PeekAny([]string{HeaderContentEncoding, HeaderContentEncoding2}))
//	if ce == "gzip" {
//		b, err := req.BodyGunzip()
//		if err != nil {
//			panic(err)
//		}
//		return b
//	} else if ce == "deflate" {
//		b, err := req.BodyInflate()
//		if err != nil {
//			panic(err)
//		}
//		return b
//	} else {
//		return req.Body()
//	}
//}
//
//func (req *Request) SetRawBody(body []byte) {
//	req.Header.Del(HeaderContentEncoding)
//	req.Header.Del(HeaderContentEncoding2)
//	req.SetBody(body)
//}
//
//func (resp *Response) GetRawBody() []byte {
//	ce := string(resp.Header.PeekAny([]string{HeaderContentEncoding, HeaderContentEncoding2}))
//	if ce == "gzip" {
//		body, err := resp.BodyGunzip()
//		if err != nil {
//			panic(err)
//		}
//		return body
//	} else if ce == "deflate" {
//		body, err := resp.BodyInflate()
//		if err != nil {
//			panic(err)
//		}
//		return body
//	} else {
//		return resp.Body()
//	}
//}

func (h *ResponseHeader) PeekAny(keys []string) []byte {
	for _, k := range keys {
		v := h.Peek(k)
		if len(v) > 0 {
			return v
		}
	}
	return nil
}

func (ctx *Request) ParseBasicAuth() (exists bool, user, pass []byte) {
	username := ctx.getURI().Username()
	if username != nil && len(username) > 0 {
		return true, username, ctx.getURI().Password()
	}

	ctx.ParseAuthorization()

	return len(ctx.getURI().Username()) > 0, ctx.getURI().Username(), ctx.getURI().Password()

}

func (ctx *Request) ParseAuthorization() (authType string) {

	key := ctx.Header.PeekAny(AuthHeaderKeys)
	if len(key) > 0 {
		kvPair := strings.Split(string(key), " ")
		if len(kvPair) == 2 {
			if kvPair[0] == "Basic" {
				authType = "Basic"

				decoded, err := base64.StdEncoding.DecodeString(kvPair[1])
				if err != nil {
					log.Errorf("parse basic auth [%v] error: %v", kvPair[1], err)
				}
				info := bytes.Split(decoded, []byte(":"))
				if len(info) == 2 {
					ctx.uri.username = info[0]
					ctx.uri.password = info[1]
				}
			} else if kvPair[0] == "ApiKey" {
				authType = "ApiKey"

				decoded, err := base64.StdEncoding.DecodeString(kvPair[1])
				if err != nil {
					log.Errorf("parse apiKey [%v] error: %v", kvPair[1], err)
				}
				info := bytes.Split(decoded, []byte(":"))
				if len(info) == 2 {
					ctx.uri.apiID = info[0]
					ctx.uri.apiKey = info[1]
				}
			}
		}
	}
	return authType
}

var AuthHeaderKeys = []string{"authorization", "Authorization", "AUTHORIZATION"}

var colon = []byte(": ")
var newLine = []byte("\n")
var contentTypes = []string{HeaderContentType2, HeaderContentType}

func getLengthBytes(data []byte) []byte {
	l := len(data)
	bytesLength := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytesLength, uint32(l))
	return bytesLength
}

func (req *Request) OverrideBodyEncode(body []byte, removeCompressHeader bool) []byte {
	req.encodeLocker.Lock()
	defer req.encodeLocker.Unlock()

	req.Header.Del("content-type")
	req.Header.Del("Content-Type")

	t := req.Header.PeekAny(contentTypes)
	if t == nil || len(t) == 0 {
		t = strJsonContentType
	}

	headerBuffer := bytes.Buffer{}
	req.Header.VisitAll(func(key, value []byte) {
		key1 := util.UnsafeBytesToString(key)
		if removeCompressHeader && (key1 == HeaderContentEncoding || key1 == HeaderContentEncoding2) {
			return
		}
		headerBuffer.Write(key)
		headerBuffer.Write(colon)
		headerBuffer.Write(value)
		headerBuffer.Write(newLine)
	})

	headerBytes := headerBuffer.Bytes()

	data := bytes.Buffer{}

	//schema
	schema := req.getURI().Scheme()
	data.Write(getLengthBytes(schema))
	data.Write(schema)

	//method
	methd := req.Header.Method()
	data.Write(getLengthBytes(methd))
	data.Write(methd)

	//uri
	uri := req.RequestURI()
	data.Write(getLengthBytes(uri))
	data.Write(uri)
	//header
	data.Write(getLengthBytes(headerBytes))
	data.Write(headerBytes)

	data.Write(getLengthBytes(body))
	if len(body) > 0 {
		data.Write(body)
	}

	b := data.Bytes()

	return b
}

func (req *Request) Encode() []byte {
	body := req.Body()
	return req.OverrideBodyEncode(body, false)
}

// peak value
func (h *RequestHeader) PeekAnyKey(keys []string) (key string, value []byte) {
	for _, k := range keys {
		v := h.Peek(k)
		if len(v) > 0 {
			return k, v
		}
	}
	return "", nil
}

type ClientAPI interface {
	Do(req *Request, resp *Response) error
	DoTimeout(req *Request, resp *Response, timeout time.Duration) error
}
