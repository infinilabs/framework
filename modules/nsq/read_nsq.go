/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nsq

import (
	"bytes"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/pipeline"
	"github.com/nsqio/go-nsq"
)

type ReadNSQJoint struct {
}

func (joint ReadNSQJoint) Name() string {
	return "read_nsq"
}

func (joint ReadNSQJoint) Process(c *pipeline.Context) error {

	return nil
}

func main() {

	cfg := nsq.NewConfig()

	// the message we'll send to ourselves
	msg := []byte("the message")

	// Set up a Producer, pointing at the default host:port
	p, err := nsq.NewProducer("localhost:4150", cfg)
	if err != nil {
		log.Error(err)
	}

	// Publish a single message to the 'embedded' topic
	err = p.Publish("embedded", msg)
	if err != nil {
		log.Error(err)
	}

	// Now set up a consumer
	c, err := nsq.NewConsumer("embedded", "local", cfg)
	if err != nil {
		log.Error(err)
	}

	// and a single handler that just checks that the message we
	// received matches the message we sent
	c.AddHandler(nsq.HandlerFunc(func(m *nsq.Message) error {
		if bytes.Compare(m.Body, msg) != 0 {
			log.Error("message didn't match:", string(m.Body))
		} else {
			log.Error("message matched:", string(m.Body))
		}
		return nil
	}))

	// Connect the consumer to the embedded nsqd instance
	c.ConnectToNSQD("localhost:4150")

}
