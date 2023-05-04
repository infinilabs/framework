/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package fasthttp

import (
	"bytes"
	"context"
	"net"
	"strings"
	"time"

	"github.com/buger/jsonparser"
	"github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/util"
)

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
	flowProcess bytes.Buffer
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
				valueStr := util.ToString(value)
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
								uri := para.Request.CloneURI()
								uri.SetPath(valueStr)
								para.Request.SetURI(uri)
								ReleaseURI(uri)
							case "body":
								para.Request.SetBodyString(valueStr)
							case "body_json":
								keys := keys[3:]
								if len(keys) == 0 {
									return nil, errors.New("invalid json key:" + s)
								}
								body := para.Request.GetRawBody()
								body, err := jsonparser.Set(body, []byte(valueStr), keys...)
								para.Request.SetRawBody(body)
								if err != nil {
									return nil, err
								}
							case "query_args":
								if len(keys) == 4 { //TODO notify
									argsField := keys[3]
									if argsField != "" {
										uri := para.Request.CloneURI()
										uri.QueryArgs().Set(argsField, valueStr)
										para.Request.SetURI(uri)
										ReleaseURI(uri)
										return nil, nil
									}
								}
							case "header":
								if len(keys) == 4 { //TODO notify
									headerKey := keys[3]
									if headerKey != "" {
										para.Request.Header.Set(headerKey, valueStr)
										return nil, nil
									}
								}
								return nil, nil
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
									return nil, err
								}
								para.Response.SetStatusCode(status)
							case "content_type":
								para.Response.Header.SetContentType(valueStr)
							case "header":
								if len(keys) == 4 { //TODO notify
									headerKey := keys[3]
									if headerKey != "" {
										para.Response.Header.Set(headerKey, valueStr)
										return nil, nil
									}
								}
							case "body":
								para.Response.SetBodyString(valueStr)
							case "body_json":

								keys := keys[3:]

								if len(keys) == 0 {
									return nil, errors.New("invalid json key:" + s)
								}

								body := para.Response.GetRawBody()

								body, err := jsonparser.Set(body, []byte(valueStr), keys...)

								para.Response.SetRawBody(body)
								if err != nil {
									return nil, err
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

var builtin = []byte("_")[0]

func (para *RequestCtx) GetValue(s string) (interface{}, error) {

	if len(s) > 0 {
		firstChar := s[0]
		if firstChar == builtin {
			keys := strings.Split(s, ".")
			firstPart := keys[0]
			switch firstPart {
			case "_ctx":
				if len(keys) >= 2 {
					rootFied := keys[1]
					if rootFied != "" {
						switch rootFied {
						case "id":
							return para.ID(), nil
						case "tls":
							return para.IsTLS(), nil
						case "remote_ip":
							return para.RemoteIP().String(), nil
						case "remote_addr":
							return para.RemoteAddr().String(), nil
						case "local_ip":
							return para.LocalIP().String(), nil
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
										return string(para.Request.getURI().String()), nil
									case "path":
										return string(para.Request.getURI().Path()), nil
									case "body":
										return string(para.Request.GetRawBody()), nil
									case "body_json":
										keys := keys[3:]
										if len(keys) == 0 {
											return nil, errors.New("invalid json key:" + s)
										}
										v, t, _, err := jsonparser.Get(para.Request.GetRawBody(), keys...)
										if err != nil {
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
												v := para.Request.getURI().QueryArgs().Peek(argsField)
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
									case "username": //alias to user
										exists, user, _ := para.Request.ParseBasicAuth()
										if exists {
											return string(user), nil
										}
										return "", nil
									case "password":
										exists, pass, _ := para.Request.ParseBasicAuth()
										if exists {
											return string(pass), nil
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
									case "body_json":
										keys := keys[3:]
										if len(keys) == 0 {
											return nil, errors.New("invalid json key:" + s)
										}
										v, t, _, err := jsonparser.Get(para.Response.GetRawBody(), keys...)
										if err != nil {
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
				break
			case "_sys":
				if len(keys) >= 2 {
					rootFied := keys[1]
					if rootFied != "" {
						switch rootFied {
						case "hostname":
							return util.GetHostName(), nil
						case "month_of_now":
							return int(util.GetLowPrecisionCurrentTime().Month()), nil
						case "weekday_of_now": //0,6
							return util.GetLowPrecisionCurrentTime().Weekday(), nil
						case "day_of_now":
							return util.GetLowPrecisionCurrentTime().Day(), nil
						case "hour_of_now":
							return util.GetLowPrecisionCurrentTime().Hour(), nil
						case "minute_of_now": //0,59
							return util.GetLowPrecisionCurrentTime().Minute(), nil
						case "second_of_now":
							return util.GetLowPrecisionCurrentTime().Second(), nil
						}
					}
				}
				break
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

func (ctx *RequestCtx) GetRequestProcess() string {
	return ctx.flowProcess.String()
}

func (ctx *RequestCtx) GetFlowProcess() string {
	return ctx.flowProcess.String()
}

func (ctx *RequestCtx) AddFlowProcess(str string) {
	if !ctx.EnrichedMetadata {
		return
	}

	if str != "" {
		if ctx.flowProcess.Len() > 0 {
			ctx.flowProcess.WriteString("->")
		}
		ctx.flowProcess.WriteString(str)
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
	ctx.reset()

}
