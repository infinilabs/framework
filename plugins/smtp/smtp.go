package smtp

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/gopkg.in/gomail.v2"
	"github.com/valyala/fasttemplate"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
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
	IdleTimeoutInSeconds int                    `config:"idle_timeout_in_seconds"`
	MessageField         param.ParaKey          `config:"message_field"`
	VariableStartTag     string                 `config:"variable_start_tag"`
	VariableEndTag       string                 `config:"variable_end_tag"`
	Variables            map[string]interface{} `config:"variables"`

	Server struct {
		Host string `config:"host"`
		Port int    `config:"port"`
		TLS  bool   `config:"tls"`
	} `config:"server"`

	Auth struct {
		Username string `config:"username"`
		Password string `config:"password"`
		SendFrom string `config:"from"`
	} `config:"auth"`

	Recipients struct {
		To  []string `config:"to"`
		CC  []string `config:"cc"`
		BCC []string `config:"bcc"`
	} `config:"recipients"`

	Templates map[string]*Template `config:"templates"`
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
		IdleTimeoutInSeconds: 0,
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

	if processor.config.Auth.SendFrom == "" {
		processor.config.Auth.SendFrom = processor.config.Auth.Username
	}

	for _, v := range processor.config.Templates {
		if v.BodyFile != "" {
			b, err := util.FileGetContent(v.BodyFile)
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
			//msg := []byte("{\"template\":\"trial_license\", \"variables\":{ \"email\":\"m@medcl.net\",\"name\":\"Medcl\",\"company\":\"INFINI Labs\",\"phone\":\"400-139-9200\"}}")
			//log.Error("message is: ", message.String())
			o := util.MapStr{}
			err := util.FromJSONBytes(message.Data, &o)
			if err != nil {
				panic(err)
			}
			//pass variables to template

			//prepare email

			//validate email
			vars := o["variables"].(map[string]interface{})
			to := vars["email"].(string)
			//name:=vars["name"].(string)

			tpName := o["template"].(string)
			tmplate, ok := processor.config.Templates[tpName]
			if !ok {
				panic(errors.Errorf("template [%v] not found", tpName))
			}
			subj := tmplate.Subject
			ctype := tmplate.ContentType
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
			err = processor.send(to, processor.config.Recipients.CC, subj, ctype, cBody, tmplate.Attachments)
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

func (processor *SMTPProcessor) send(to string, ccs []string, subject, contentType, body string, attachments []Attachment) error {

	//log.Error(to,ccs,subject,contentType,body,processor.config.Server.TLS,processor.config.Server.Host, processor.config.Server.Port, processor.config.Auth.SendFrom,processor.config.Auth.Username, processor.config.Auth.Password)

	// Create a new message
	message := gomail.NewMessage()
	message.SetHeader("From", processor.config.Auth.SendFrom)
	message.SetHeader("To", to)

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

	d := gomail.NewDialerWithTimeout(processor.config.Server.Host, processor.config.Server.Port, processor.config.Auth.Username, processor.config.Auth.Password, time.Duration(processor.config.DialTimeoutInSeconds)*time.Second)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	d.SSL = processor.config.Server.TLS
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
