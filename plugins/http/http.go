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

package http

//curl -XPOST gateway_endpoint/message_replicate/:queue_name/ -d'BYTES_OF_MESSAGE_DATA'

import (
	"fmt"
	"io"
	"strings"
	"time"

	log "github.com/cihub/seelog"
	rate2 "golang.org/x/time/rate"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
	"infini.sh/framework/lib/fasttemplate"
)

// Default threshold in bytes to enable gzip compression if not configured
const defaultCompressionThreshold = 1024

type HTTPProcessor struct {
	config       *Config
	client       *fasthttp.Client
	pathTemplate *fasttemplate.Template //path template
	rater        *rate2.Limiter
	httpPool     *fasthttp.RequestResponsePool
}

func (processor *HTTPProcessor) Name() string {
	return "http"
}

type Config struct {
	MessageField param.ParaKey `config:"message_field"`

	Schema    string            `config:"schema"`     //support variable
	Hosts     []string          `config:"hosts"`      //support variable
	Method    string            `config:"method"`     //support variable
	Path      string            `config:"path"`       //support variable
	Headers   map[string]string `config:"headers"`    //support variable
	BasicAuth *model.BasicAuth  `config:"basic_auth"` //support variable
	TLSConfig *config.TLSConfig `config:"tls"`        //client tls config

	Compress             bool `config:"compress"`              // compress request body, default false
	CompressionThreshold int  `config:"compression_threshold"` // default 1024 bytes

	ValidatedStatusCode []int `config:"valid_status_code"` //validated status code, default 200

	//host
	MaxSendingQPS       int `config:"max_sending_qps"`
	MaxConnection       int `config:"max_connection_per_node"`
	MaxResponseBodySize int `config:"max_response_size"`
	MaxRetryTimes       int `config:"max_retry_times"`
	RetryDelayInMs      int `config:"retry_delay_in_ms"`

	MaxConnWaitTimeout  time.Duration `config:"max_conn_wait_timeout"`
	MaxIdleConnDuration time.Duration `config:"max_idle_conn_duration"`
	MaxConnDuration     time.Duration `config:"max_conn_duration"`
	Timeout             time.Duration `config:"timeout"`
	ReadTimeout         time.Duration `config:"read_timeout"`
	WriteTimeout        time.Duration `config:"write_timeout"`
	ReadBufferSize      int           `config:"read_buffer_size"`
	WriteBufferSize     int           `config:"write_buffer_size"`
}

func init() {
	pipeline.RegisterProcessorPlugin("http", New)
}

func New(c *config.Config) (pipeline.Processor, error) {
	cfg := Config{
		MessageField:         "messages",
		ValidatedStatusCode:  []int{200, 201},
		Timeout:              10 * time.Second,
		ReadTimeout:          10 * time.Second,
		WriteTimeout:         10 * time.Second,
		MaxIdleConnDuration:  10 * time.Second,
		MaxConnWaitTimeout:   10 * time.Second,
		Compress:             false,
		CompressionThreshold: defaultCompressionThreshold,
	}

	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of http_replicator processor: %s", err)
	}

	processor := &HTTPProcessor{
		config:   &cfg,
		httpPool: fasthttp.NewRequestResponsePool("http_processor_" + util.GetUUID()),
	}

	processor.client = &fasthttp.Client{
		Name:                          "reverse_proxy",
		DisableHeaderNamesNormalizing: true,
		DisablePathNormalizing:        true,
		MaxConnsPerHost:               processor.config.MaxConnection,
		MaxResponseBodySize:           processor.config.MaxResponseBodySize,
		MaxConnWaitTimeout:            processor.config.MaxConnWaitTimeout,
		MaxConnDuration:               processor.config.MaxConnDuration,
		MaxIdleConnDuration:           processor.config.MaxIdleConnDuration,
		ReadTimeout:                   processor.config.ReadTimeout,
		WriteTimeout:                  processor.config.WriteTimeout,
		ReadBufferSize:                processor.config.ReadBufferSize,
		WriteBufferSize:               processor.config.WriteBufferSize,
		DialDualStack:                 true,
		TLSConfig:                     api.SimpleGetTLSConfig(processor.config.TLSConfig),
	}

	var err error
	if strings.Contains(processor.config.Path, "$[[") {
		processor.pathTemplate, err = fasttemplate.NewTemplate(processor.config.Path, "$[[", "]]")
		if err != nil {
			panic(err)
		}
	}

	if processor.config.MaxSendingQPS > 0 {
		processor.rater = rate.GetRateLimiter("http_replicator", "sending", processor.config.MaxSendingQPS, processor.config.MaxSendingQPS, time.Second)
	}

	return processor, nil
}

func (processor *HTTPProcessor) Process(ctx *pipeline.Context) error {

	req := processor.httpPool.AcquireRequestWithTag("http_processor")
	resp := processor.httpPool.AcquireResponseWithTag("http_processor")

	defer processor.httpPool.ReleaseRequest(req)
	defer processor.httpPool.ReleaseResponse(resp)

	path := processor.config.Path
	if processor.pathTemplate != nil {
		path = processor.pathTemplate.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
			variable, err := ctx.GetValue(tag)
			if err == nil {
				return w.Write([]byte(util.ToString(variable)))
			}
			return -1, err
		})
	}
	for key, value := range processor.config.Headers {
		req.Header.Set(key, value)
	}

	uri := req.CloneURI()
	uri.SetPath(path)
	uri.SetScheme(processor.config.Schema)

	req.SetURI(uri)
	req.Header.SetMethod(processor.config.Method)

	if processor.config.BasicAuth != nil {
		req.SetBasicAuth(processor.config.BasicAuth.Username, processor.config.BasicAuth.Password.Get())
	}

	//get message from queue
	obj := ctx.Get(processor.config.MessageField)
	if obj != nil {
		messages := obj.([]queue.Message)
		log.Tracef("get %v messages from context", len(messages))
		if len(messages) == 0 {
			return nil
		}
		//parse template
		for _, message := range messages {

			if global.ShuttingDown() {
				panic(errors.Errorf("shutting down"))
			}

			req.ResetBody()
			resp.ResetBody()

			// --- Prepare Request Body (Compress if enabled and size meets threshold) ---
			if processor.config.Compress && len(message.Data) >= processor.config.CompressionThreshold {
				_, err := fasthttp.WriteGzipLevel(req.BodyWriter(), message.Data, fasthttp.CompressBestCompression)
				if err != nil {
					panic(errors.Errorf("failed to compress message: %v", err))
				}
				req.Header.Set(fasthttp.HeaderContentEncoding, "gzip")

				log.Tracef("Message size %d >= threshold %d, compressed message size to %d", len(message.Data), processor.config.CompressionThreshold, req.GetBodyLength())

			} else {
				req.SetBody(message.Data)
				// This prevents sending incorrect headers if the pooled 'req' object had it set previously.
				req.Header.Del(fasthttp.HeaderContentEncoding)
			}

			var success = false
			for _, v := range processor.config.Hosts {

				if global.ShuttingDown() {
					panic(errors.Errorf("shutting down"))
				}

				req.SetHost(v)

				if global.ShuttingDown() {
					break
				}

				if processor.rater != nil {
					if !processor.rater.Allow() {
						time.Sleep(100 * time.Millisecond)
					}
				}

				err := processor.client.DoTimeout(req, resp, processor.config.Timeout)
				if err != nil {
					log.Error(v, ",", err)
					continue
				}
				if !util.ContainsInAnyInt32Array(resp.StatusCode(), processor.config.ValidatedStatusCode) {
					log.Tracef("http request validated failed, status code: %d, %v, %v", resp.StatusCode(), req.String(), resp.String())
					panic(errors.Errorf("http request validated failed, status code: %d, req content length %v, resp content length %v", resp.StatusCode(), req.Header.ContentLength(), resp.Header.ContentLength()))
				}
				success = true
				break
			}
			if !success {
				log.Tracef("http request failed, status code: %d, %v, %v", resp.StatusCode(), req.String(), resp.String())
				panic(errors.Errorf("http request failed, status code: %d, req content length %v, resp content length %v", resp.StatusCode(), req.Header.ContentLength(), resp.Header.ContentLength()))
			}
		}
	}
	return nil
}
