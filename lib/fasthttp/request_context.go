/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package fasthttp

import (
	"context"
	"github.com/buger/jsonparser"
	"github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/util"
	"net"
	"strings"
	"time"
)

// RequestCtx contains incoming request and manages outgoing response.
//
// It is forbidden copying RequestCtx instances.
//
// RequestHandler should avoid holding references to incoming RequestCtx and/or
// its' members after the return.
// If holding RequestCtx references after the return is unavoidable
// (for instance, ctx is passed to a separate goroutine and ctx lifetime cannot
// be controlled), then the RequestHandler MUST call ctx.TimeoutError()
// before return.
//
// It is unsafe modifying/reading RequestCtx instance from concurrently
// running goroutines. The only exception is TimeoutError*, which may be called
// while other goroutines accessing RequestCtx.
//type RequestCtx struct {
//	noCopy noCopy //nolint:unused,structcheck
//
//	param.Parameters
//
//	// Incoming request.
//	//
//	// Copying Request by value is forbidden. Use pointer to Request instead.
//	Request Request
//
//	// Outgoing response.
//	//
//	// Copying Response by value is forbidden. Use pointer to Response instead.
//	Response Response
//
//	userValues userData
//
//	connID         uint64
//	connRequestNum uint64
//	connTime       time.Time
//
//	time time.Time
//
//	s   *Server
//	c   net.Conn
//	fbr firstByteReader
//
//	timeoutResponse *Response
//	timeoutCh       chan struct{}
//	timeoutTimer    *time.Timer
//
//	hijackHandler    HijackHandler
//	hijackNoResponse bool
//
//	finished bool
//	//SequenceID       int64
//	flowProcess []string
//	destination []string
//
//	EnrichedMetadata bool
//
//	// ctx is either the client or server context. It should only
//	// be modified via copying the whole Request using WithContext.
//	// It is unexported to prevent people from using Context wrong
//	// and mutating the contexts held by callers of the same request.
//	ctx context.Context
//
//}

type RequestCtx struct {
	param.Parameters
	ctx context.Context

	noCopy noCopy //nolint:unused,structcheck

	// Incoming request.
	//
	// Copying Request by value is forbidden. Use pointer to Request instead.
	Request Request

	// Outgoing response.
	//
	// Copying Response by value is forbidden. Use pointer to Response instead.
	Response Response

	userValues userData

	connID         uint64
	connRequestNum uint64
	connTime       time.Time
	remoteAddr     net.Addr

	time time.Time

	logger ctxLogger
	s      *Server
	c      net.Conn
	fbr    firstByteReader

	timeoutResponse *Response
	timeoutCh       chan struct{}
	timeoutTimer    *time.Timer

	hijackHandler    HijackHandler
	hijackNoResponse bool
	finished         bool

	//SequenceID       int64
	flowProcess []string
	destination []string

	EnrichedMetadata bool
}


// Context returns the request's context. To change the context, use
// WithContext.
//
// The returned context is always non-nil; it defaults to the
// background context.
//
// For outgoing client requests, the context controls cancellation.
//
// For incoming server requests, the context is canceled when the
// client's connection closes, the request is canceled (with HTTP/2),
// or when the ServeHTTP method returns.
func (r *RequestCtx) Context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}


func (para *RequestCtx) PutValue(s string, value interface{}) (interface{}, error) {

	if util.PrefixStr(s, "_ctx.") {
		keys := strings.Split(s, ".")
		if len(keys) >= 2 {
			rootFied := keys[1]
			if rootFied != "" {
				valueStr:=util.ToString(value)
				switch rootFied {
				case "request":
					if len(keys) >= 3 {
						requestField := keys[2]
						if requestField != "" {
							switch requestField {
							case "host":
								para.Request.SetHost(valueStr)
							case "method":
								para.Request.Header.SetMethod(valueStr)
							case "uri":
								para.Request.SetRequestURI(valueStr)
							case "path":
								para.Request.URI().SetPath(valueStr)
							case "body":
								para.Request.SetBodyString(valueStr)
							case "body_json":
								keys := keys[3:]
								if len(keys) == 0 {
									return nil,errors.New("invalid json key:" + s)
								}
								body := para.Request.GetRawBody()
								body, err := jsonparser.Set(body, []byte(valueStr), keys...)
								para.Request.SetRawBody(body)
								if err != nil {
									return nil,err
								}
							case "query_args":
								if len(keys) == 4 { //TODO notify
									argsField := keys[3]
									if argsField != "" {
										para.Request.URI().QueryArgs().Set(argsField, valueStr)
										return nil,nil
									}
								}
							case "header":
								if len(keys) == 4 { //TODO notify
									headerKey := keys[3]
									if headerKey != "" {
										para.Request.Header.Set(headerKey, valueStr)
										return nil,nil
									}
								}
								return nil,nil
							}
						}
					}
					break
				case "response":
					if len(keys) >= 3 {
						responseField := keys[2]
						if responseField != "" {
							switch responseField {
							case "status":
								status, err := util.ToInt(valueStr)
								if err != nil {
									return nil,err
								}
								para.Response.SetStatusCode(status)
							case "content_type":
								para.Response.Header.SetContentType(valueStr)
							case "header":
								if len(keys) == 4 { //TODO notify
									headerKey := keys[3]
									if headerKey != "" {
										para.Response.Header.Set(headerKey, valueStr)
										return nil,nil
									}
								}
							case "body":
								para.Response.SetBodyString(valueStr)
							case "body_json":

								keys := keys[3:]

								if len(keys) == 0 {
									return nil,errors.New("invalid json key:" + s)
								}

								body := para.Response.GetRawBody()

								body, err := jsonparser.Set(body, []byte(valueStr), keys...)

								para.Response.SetBodyRaw(body)
								if err != nil {
									return nil,err
								}
							}
						}
					}
					break
				}
			}
		}
	}

	return para.Parameters.PutValue(s, value)
}

func (para *RequestCtx) GetValue(s string) (interface{}, error) {

	if util.PrefixStr(s, "_ctx.") {
		keys := strings.Split(s, ".")
		if len(keys) >= 2 {
			rootFied := keys[1]
			if rootFied != "" {
				switch rootFied {
				case "id":
					return para.ID(), nil
				case "tls":
					return para.IsTLS(), nil
				case "remote_addr":
					return para.RemoteAddr().String(), nil
				case "local_addr":
					return para.LocalAddr().String(), nil
				case "elapsed":
					return para.GetElapsedTime().Milliseconds(), nil
				case "request":
					if len(keys) >= 3 {
						requestField := keys[2]
						if requestField != "" {
							switch requestField {
							case "to_string":
								return string(para.Request.String()), nil
							case "host":
								return string(para.Request.Host()), nil
							case "method":
								return string(para.Method()), nil
							case "uri":
								return string(para.Request.URI().String()), nil
							case "path":
								return string(para.Request.URI().Path()), nil
							case "body":
								return string(para.Request.GetRawBody()), nil
							case "body_json":
								keys := keys[3:]
								if len(keys) == 0 {
									return nil, errors.New("invalid json key:" + s)
								}
								v, t, _, err := jsonparser.Get(para.Request.GetRawBody(), keys...)
								if err != nil {
									//log.Error(s,err)
									return nil, err
								}

								switch t {
								case jsonparser.NotExist:
									return nil, errors.New("key not found:" + s)
								case jsonparser.String:
									return string(v), nil
								case jsonparser.Boolean:
									return jsonparser.ParseBoolean(v)
								case jsonparser.Number:
									i, err := jsonparser.ParseInt(v)
									if err != nil {
										return jsonparser.ParseFloat(v)
									}
									return i, err
								default:
									seelog.Error("json type not handled:", s, v, t, err)
								}

							case "body_length":
								return para.Request.GetBodyLength(), nil
							case "body_size":
								return para.Request.GetBodyLength(), nil
							case "query_args":
								if len(keys) == 4 { //TODO notify
									argsField := keys[3]
									if argsField != "" {
										v := para.Request.URI().QueryArgs().Peek(argsField)
										return string(v), nil
									}
								}
							case "header":
								if len(keys) == 4 { //TODO notify
									headerKey := keys[3]
									if headerKey != "" {
										v := para.Request.Header.Peek(headerKey)
										return string(v), nil
									}
								}
							case "user":
								exists, user, _ := para.Request.ParseBasicAuth()
								if exists {
									return string(user), nil
								}
								return "", nil
							}
						}
					}
					break
				case "response":
					if len(keys) >= 3 {
						responseField := keys[2]
						if responseField != "" {
							switch responseField {
							case "to_string":
								return para.Response.String(), nil
							case "status":
								return para.Response.StatusCode(), nil
							case "status_code":
								return para.Response.StatusCode(), nil
							case "body":
								return string(para.Response.GetRawBody()), nil
							case "content_type":
								return string(para.Response.Header.ContentType()), nil
							case "body_length":
								return para.Response.GetBodyLength(), nil
							case "body_size":
								return para.Response.GetBodyLength(), nil
							case "header":
								if len(keys) == 4 { //TODO notify
									headerKey := keys[3]
									if headerKey != "" {
										v := para.Response.Header.Peek(headerKey)
										return string(v), nil
									}
								}
							}
						}
					}
					break
				}
			}
		}
	}

	v := para.Get(param.ParaKey(s))
	if v != nil {
		return v, nil
	} else {
		return nil, errors.New("key not found:" + s)
	}
}


func (ctx *RequestCtx) GetRequestProcess() []string {
	return ctx.flowProcess
}

func (ctx *RequestCtx) GetFlowProcess() []string {
	return ctx.flowProcess
}

func (ctx *RequestCtx) AddFlowProcess(str string) {
	if !ctx.EnrichedMetadata {
		return
	}

	if str != "" {
		ctx.flowProcess = append(ctx.flowProcess, str)
	}
}

func (ctx *RequestCtx) SetDestination(str string) {
	ctx.destination = append(ctx.destination, str)
}

func (ctx *RequestCtx) Destination() []string {
	return ctx.destination
}


func (ctx *RequestCtx) Reset() {
	//reset flags and metadata
	if ctx.Data == nil || len(ctx.Data) > 0 {
		ctx.ResetParameters()
		ctx.Parameters.ResetParameters()
	}
	ctx.finished = false
	ctx.flowProcess = []string{}
	ctx.destination = ctx.destination[0:0]
	ctx.userValues.Reset()

}