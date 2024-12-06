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

package replay

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"path"
	"runtime"
	"strings"
	time2 "time"

	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/config"
	"github.com/rubyniu105/framework/core/errors"
	"github.com/rubyniu105/framework/core/global"
	"github.com/rubyniu105/framework/core/pipeline"
	"github.com/rubyniu105/framework/core/progress"
	"github.com/rubyniu105/framework/core/util"
	"github.com/rubyniu105/framework/lib/bytebufferpool"
	"github.com/rubyniu105/framework/lib/fasthttp"
)

type Config struct {
	Schema string `config:"schema"`
	Host   string `config:"host"`

	Filename   string `config:"filename"`
	InputQueue string `config:"input_queue"`
	Username   string `config:"username"`
	Password   string `config:"password"`
}

type ReplayProcessor struct {
	config   *Config
	HTTPPool *fasthttp.RequestResponsePool
}

var signalChannel = make(chan bool, 1)

func init() {
	pipeline.RegisterProcessorPlugin("replay", New)
}

func New(c *config.Config) (pipeline.Processor, error) {
	cfg := Config{
		Schema: "http",
		Host:   "localhost:9200",
	}

	if err := c.Unpack(&cfg); err != nil {
		log.Error(err)
		return nil, fmt.Errorf("failed to unpack the configuration of flow_runner processor: %s", err)
	}

	runner := ReplayProcessor{config: &cfg}
	runner.HTTPPool = fasthttp.NewRequestResponsePool("replay_filter_" + util.GetUUID())

	return &runner, nil
}

func (processor ReplayProcessor) Stop() error {
	signalChannel <- true
	return nil
}

func (processor *ReplayProcessor) Name() string {
	return "replay"
}

var validVerbs = []string{
	fasthttp.MethodGet,
	fasthttp.MethodPut,
	fasthttp.MethodPost,
	fasthttp.MethodDelete,
}
var commentMarks = []string{
	"#", "//",
}

var fastHttpClient = &fasthttp.Client{
	MaxConnsPerHost:               1000,
	Name:                          "replay",
	ReadTimeout:                   10 * time2.Second,
	WriteTimeout:                  10 * time2.Second,
	MaxConnWaitTimeout:            10 * time2.Second,
	DisableHeaderNamesNormalizing: false,
	TLSConfig:                     &tls.Config{InsecureSkipVerify: true},
	DialDualStack:                 true,
}

const newline = "\n"

func (processor *ReplayProcessor) Process(ctx *pipeline.Context) error {
	defer func() {
		if !global.Env().IsDebug {
			if r := recover(); r != nil {
				var v string
				switch r.(type) {
				case error:
					v = r.(error).Error()
				case runtime.Error:
					v = r.(runtime.Error).Error()
				case string:
					v = r.(string)
				}
				log.Errorf("error in flow_runner [%v], [%v]", processor.Name(), v)
				ctx.RecordError(fmt.Errorf("replay processor panic: %v", r))
			}
		}
	}()
	var count int
	time := time2.Now()

	if processor.config.Filename != "" {

		filename := processor.config.Filename
		if !util.FileExists(filename) && !util.PrefixStr(filename, "/") {
			filename = path.Join(global.Env().GetDataDir(), filename)
		}

		lines := util.FileGetLines(filename)

		log.Debugf("get %v lines prepare to replay", len(lines))

		var err error
		var done bool

		req := processor.HTTPPool.AcquireRequest()
		res := processor.HTTPPool.AcquireResponse()

		defer processor.HTTPPool.ReleaseRequest(req)
		defer processor.HTTPPool.ReleaseResponse(res)

		count, err, done = ReplayLines(req, res, ctx, lines, processor.config.Schema, processor.config.Host, processor.config.Username, processor.config.Password)
		if done {
			return err
		}

		progress.Stop()
	}

	if count > 0 {
		log.Infof("finished replay [%v] requests, elapsed: %v", count, time2.Since(time).String())
	}

	return nil
}

func ReplayLines(req *fasthttp.Request, res *fasthttp.Response, ctx *pipeline.Context, lines []string, schema, host, username, password string) (int, error, bool) {

	var buffer = bytebufferpool.Get("replay")
	defer bytebufferpool.Put("replay", buffer)

	var requestIsSet bool
	count := 0
	for _, line := range lines {
		count++
		if ctx.IsCanceled() {
			return 0, nil, true
		}

		line = strings.TrimSpace(line)
		if line != "" {

			//skip comments
			if util.PrefixAnyInArray(line, commentMarks) {
				continue
			}

			//if start with GET/POST etc, it's mew request start
			//buffer is empty, start new request
			//buffer is not empty, clear current request first
			if util.PrefixAnyInArray(line, validVerbs) {

				//execute previous request now
				if requestIsSet {
					log.Debug("execute request: ", req.PhantomURI().String())
					if username != "" && password != "" {
						req.SetBasicAuth(username, password)
					}
					err := execute(req, res, buffer)
					if err != nil {
						log.Error(err, req.String())
						panic(err)
					}
					buffer.Reset()
					requestIsSet = false
				}

				//prepare new request
				arr := strings.Fields(line)
				if len(arr) >= 2 {
					method := arr[0]
					uri := arr[1]
					req.SetRequestURI(uri)
					clonedURI := req.CloneURI()
					req.Header.SetMethod(method)
					req.Header.SetHost(host)
					clonedURI.SetScheme(schema)
					clonedURI.SetHost(host)
					req.SetURI(clonedURI)
					fasthttp.ReleaseURI(clonedURI)
					req.SetHost(host)

					if global.Env().IsDebug {
						log.Trace(req.String())
					}

					requestIsSet = true
				} else {
					panic(errors.Errorf("request meta is not valid : %v", line))
				}
			} else {
				if requestIsSet {
					if buffer.Len() > 0 {
						buffer.WriteString(newline)
					}
					buffer.WriteString(line)
				} else {
					panic(errors.Errorf("request meta is not set, but found body: %v", line))
				}

			}
		}
	}

	//execute previous request now
	if requestIsSet {
		log.Debug("execute last request: ", req.PhantomURI().String())
		if username != "" && password != "" {
			req.SetBasicAuth(username, password)
		}
		err := execute(req, res, buffer)
		if err != nil {
			log.Error(err, req.String())
			panic(err)
		}
	}

	return count, nil, false
}

func execute(req *fasthttp.Request, res *fasthttp.Response, buffer *bytebufferpool.ByteBuffer) error {
	defer func() {
		req.Reset()
		res.Reset()
	}()
	if buffer.Len() > 0 {
		if util.ContainStr(string(req.Header.RequestURI()), "_bulk") {
			buffer.WriteString(newline)
		}
		req.SetBody(buffer.Bytes())
	}

	err := fastHttpClient.Do(req, res)
	if err != nil {
		return err
	}

	if res.StatusCode() > 210 {
		if global.Env().IsDebug {
			log.Debug("request:", string(req.String()))
			log.Debug("response:", string(res.String()))
		}
		method := strings.ToUpper(string(req.Header.Method()))
		if res.StatusCode() != 404 || method != http.MethodDelete {
			return fmt.Errorf("%s %s \n%s", method, string(req.Header.RequestURI()), string(res.GetRawBody()))
		}
	}

	if global.Env().IsDebug {
		log.Trace(string(res.GetRawBody()))
	}
	return nil
}
