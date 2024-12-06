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

package smtp

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/gopkg.in/gomail.v2"
	"github.com/rubyniu105/framework/core/config"
	"github.com/rubyniu105/framework/core/errors"
	"github.com/rubyniu105/framework/core/global"
	"github.com/rubyniu105/framework/core/param"
	"github.com/rubyniu105/framework/core/pipeline"
	"github.com/rubyniu105/framework/core/queue"
	"github.com/rubyniu105/framework/core/util"
	"github.com/valyala/fasttemplate"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

type SMTPProcessor struct {
	config *Config
}

func (processor *SMTPProcessor) Name() string {
	return "smtp"
}

type Template struct {
	ContentType       string       `config:"content_type"` //text or html
	Subject           string       `config:"subject"`
	Body              string       `config:"body"`
	BodyFile          string       `config:"body_file"` //use file to store template
	Attachments       []Attachment `config:"attachments"`
	variableInSubject bool         //this template contains variable
	variableInBody    bool         //this template contains variable
	bodyTemplate      *fasttemplate.Template
	subjectTemplate   *fasttemplate.Template
}

type Config struct {
	DialTimeoutInSeconds int                    `config:"dial_timeout_in_seconds"`
	MessageField         param.ParaKey          `config:"message_field"`
	VariableStartTag     string                 `config:"variable_start_tag"`
	VariableEndTag       string                 `config:"variable_end_tag"`
	Variables            map[string]interface{} `config:"variables"`

	Servers map[string]*ServerConfig `config:"servers"`

	Templates map[string]*Template `config:"templates"`
}

type ServerConfig struct {
	Server struct {
		Host string `config:"host"`
		Port int    `config:"port"`
		TLS  bool   `config:"tls"`
	} `config:"server"`

	Auth struct {
		Username string `config:"username"`
		Password string `config:"password"`
	} `config:"auth"`

	SendFrom string `config:"sender"`

	Recipients struct {
		To  []string `config:"to"`
		CC  []string `config:"cc"`
		BCC []string `config:"bcc"`
	} `config:"recipients"`
}

type Attachment struct {
	File        string `config:"file"`
	ContentType string `config:"content_type"`
	Inline      bool   `config:"inline"`
	CID         string `config:"cid"`
}

func init() {
	pipeline.RegisterProcessorPlugin("smtp", New)
}

func New(c *config.Config) (pipeline.Processor, error) {
	cfg := Config{
		DialTimeoutInSeconds: 30,
		VariableStartTag:     "$[[",
		VariableEndTag:       "]]",
		MessageField:         "messages",
	}

	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of index_diff processor: %s", err)
	}

	processor := &SMTPProcessor{
		config: &cfg,
	}

	for _, v := range processor.config.Servers {
		if v.Auth.Username == "" && v.SendFrom != "" {
			v.Auth.Username = v.SendFrom
		}

		if v.SendFrom == "" && v.Auth.Username != "" {
			v.SendFrom = v.Auth.Username
		}
	}

	for _, v := range processor.config.Templates {
		if v.BodyFile != "" {
			file, _ := filepath.Abs(v.BodyFile)
			b, err := util.FileGetContent(file)
			if err != nil {
				panic(err)
			}
			v.Body = string(b)
		}
	}

	for _, v := range processor.config.Templates {
		if util.ContainStr(v.Body, processor.config.VariableStartTag) {
			v.variableInBody = true
			template, err := fasttemplate.NewTemplate(v.Body, processor.config.VariableStartTag, processor.config.VariableEndTag)
			if err != nil {
				panic(err)
			}
			v.bodyTemplate = template
		}
		if util.ContainStr(v.Subject, processor.config.VariableStartTag) {
			v.variableInSubject = true
			template, err := fasttemplate.NewTemplate(v.Subject, processor.config.VariableStartTag, processor.config.VariableEndTag)
			if err != nil {
				panic(err)
			}
			v.subjectTemplate = template
		}
	}

	return processor, nil

}

func (processor *SMTPProcessor) Process(ctx *pipeline.Context) error {

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
			o := util.MapStr{}
			err := util.FromJSONBytes(message.Data, &o)
			if err != nil {
				panic(err)
			}
			//pass variables to template

			//prepare email

			//validate email
			vars := o["variables"].(map[string]interface{})
			var srvCfg *ServerConfig
			var serverID string
			var ok bool
			if serverID, ok = o["server_id"].(string); !ok {
				v, ok := processor.config.Variables["server_id"]
				if v != nil {
					serverID, ok = v.(string)
				}
				if !ok || serverID == "" {
					panic(errors.Errorf("server_id is empty"))
				}
			}

			srvCfg, ok = processor.config.Servers[serverID]
			if !ok {
				panic(errors.Errorf("server_id [%v] not found", serverID))
			}

			sendTo := []string{}
			to, ok := o["email"].(string)
			if ok && to != "" {
				sendTo = append(sendTo, to)
			} else {
				to1, ok := o["email"].([]interface{})
				if ok {
					for _, v := range to1 {
						if vs, ok := v.(string); ok && vs != "" {
							sendTo = append(sendTo, vs)
						}
					}
				} else {
					email, ok := vars["email"].(string)
					if ok && email != "" {
						sendTo = append(sendTo, email)
					}
				}
			}

			if len(srvCfg.Recipients.To) > 0 {
				sendTo = append(sendTo, srvCfg.Recipients.To...)
			}

			if global.Env().IsDebug {
				log.Tracef("send to: %v", sendTo)
			}

			tpName := o["template"].(string)
			tmplate, ok := processor.config.Templates[tpName]
			if !ok {
				panic(errors.Errorf("template [%v] not found", tpName))
			}
			subj := tmplate.Subject
			ctype := tmplate.ContentType
			if contentType, ok := vars["content_type"].(string); ok && contentType != "" {
				ctype = contentType
			}
			cc := srvCfg.Recipients.CC
			switch vars["cc"].(type) {
			case string:
				cc = append(cc, vars["cc"].(string))
			case []interface{}:
				ccArr := vars["cc"].([]interface{})
				for _, cv := range ccArr {
					if v, ok := cv.(string); ok {
						cc = append(cc, v)
					}
				}
			}

			cBody := tmplate.Body

			myctx := util.MapStr{}
			myctx.Merge(processor.config.Variables)
			myctx.Merge(vars)

			//render template
			if tmplate.variableInSubject && tmplate.subjectTemplate != nil {
				subj = tmplate.subjectTemplate.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
					variable, err := myctx.GetValue(tag)
					x, ok := variable.(string)
					if ok {
						if x != "" {
							return w.Write([]byte(x))
						}
					}
					return -1, err
				})
			}
			if tmplate.variableInBody && tmplate.bodyTemplate != nil {
				cBody = tmplate.bodyTemplate.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
					variable, err := myctx.GetValue(tag)
					x, ok := variable.(string)
					if ok {
						if x != "" {
							return w.Write([]byte(x))
						}
					}
					return -1, err
				})
			}

			//send email
			err = processor.send(srvCfg, sendTo, cc, subj, ctype, cBody, tmplate.Attachments)
			if err != nil {
				panic(err)
			}
		}
	}
	return nil
}

func AddCC(msg *gomail.Message, ccs []map[string]string) {

	if len(ccs) == 0 {
		return
	}

	list := []string{}
	for _, cc := range ccs {
		for address, name := range cc {
			list = append(list, msg.FormatAddress(address, name))
		}
	}

	msg.SetHeader("Cc", list...)

}

func (processor *SMTPProcessor) send(srvCfg *ServerConfig, to []string, ccs []string, subject, contentType, body string, attachments []Attachment) error {

	if len(to) == 0 {
		return errors.New("no recipient found")
	}

	// Create a new message
	message := gomail.NewMessage()
	message.SetHeader("From", srvCfg.SendFrom)
	message.SetHeader("To", to...)

	if len(ccs) > 0 {
		message.SetHeader("Cc", ccs...)
	}

	message.SetHeader("Subject", subject)

	// Add HTML content to the message
	message.SetBody(contentType, body)

	// Attach the image
	for _, attachment := range attachments {
		h := map[string][]string{
			"Content-ID":          {attachment.CID},
			"Content-Type":        {attachment.ContentType},
			"Content-Disposition": {"attachment; filename=\"" + filepath.Base(attachment.File) + "\""},
		}
		message.Embed(attachment.File, gomail.SetHeader(h))
	}

	d := gomail.NewDialerWithTimeout(srvCfg.Server.Host, srvCfg.Server.Port, srvCfg.Auth.Username, srvCfg.Auth.Password, time.Duration(processor.config.DialTimeoutInSeconds)*time.Second)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	d.SSL = srvCfg.Server.TLS
	// Send the email to Bob, Cora and Dan.

	return d.DialAndSend(message)
}

func getImageData(imagePath string) (string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return "", err
	}

	encodedData := base64.StdEncoding.EncodeToString(data)
	return encodedData, nil
}
