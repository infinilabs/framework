/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package http

//curl -XPOST gateway_endpoint/message_replicate/:queue_name/ -d'BYTES_OF_MESSAGE_DATA'

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
	"infini.sh/framework/lib/fasttemplate"
	"io"
	"strings"
	"time"
)

type HTTPProcessor struct {
	config       *Config
	client       *fasthttp.Client
	pathTemplate *fasttemplate.Template //path template

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

	ValidatedStatusCode []int `config:"valid_status_code"` //validated status code, default 200

	//host
	MaxConnection       int `config:"max_connection_per_node"`
	MaxResponseBodySize int `config:"max_response_size"`
	MaxRetryTimes       int `config:"max_retry_times"`
	RetryDelayInMs      int `config:"retry_delay_in_ms"`

	SkipCleanupHopHeaders bool `config:"skip_cleanup_hop_headers"`
	SkipEnrichMetadata    bool `config:"skip_metadata_enrich"`

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
		MessageField:        "messages",
		ValidatedStatusCode: []int{200, 201},
	}

	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of http_replicator processor: %s", err)
	}

	processor := &HTTPProcessor{
		config: &cfg,
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
		TLSConfig:                     api.GetFastHTTPClientTLSConfig(processor.config.TLSConfig),
	}

	var err error
	if strings.Contains(processor.config.Path, "$[[") {
		processor.pathTemplate, err = fasttemplate.NewTemplate(processor.config.Path, "$[[", "]]")
		if err != nil {
			panic(err)
		}
	}

	return processor, nil
}

func (processor *HTTPProcessor) Process(ctx *pipeline.Context) error {

	req := fasthttp.AcquireRequestWithTag("http_processor")
	resp := fasthttp.AcquireResponseWithTag("http_processor")

	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

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

	uri := req.CloneURI()
	uri.SetPath(path)
	uri.SetScheme(processor.config.Schema)

	req.SetURI(uri)
	req.Header.SetMethod(processor.config.Method)

	if processor.config.BasicAuth != nil {
		req.SetBasicAuth(processor.config.BasicAuth.Username, processor.config.BasicAuth.Password)
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
			req.ResetBody()
			resp.ResetBody()
			req.SetBody(message.Data)

			var success=false
			for _,v:=range processor.config.Hosts{
				req.SetHost(v)
				err := processor.client.DoTimeout(req, resp, processor.config.Timeout)
				if err != nil {
					log.Error(v,",",err)
					continue
				}
				if !util.ContainsInAnyInt32Array(resp.StatusCode(),processor.config.ValidatedStatusCode){
					panic(errors.Errorf("http request failed, status code: %d", resp.StatusCode()))
				}
				success=true
				break
			}
			if !success{
				panic(errors.Errorf("http request failed, status code: %d", resp.StatusCode()))
			}
		}
	}
	return nil
}
