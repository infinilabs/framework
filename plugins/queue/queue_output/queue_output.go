/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

// Package queue_output provides the "queue_output" pipeline processor: the
// chain-tail companion of "consumer". It takes the message batch the
// consumer exposed in the context (typically after a for_each transform
// chain) and appends every record onto a target queue, enabling two-stage
// pipelines: process on one queue, sink (e.g. bulk_indexing) from another.
//
// Configuration:
//
//	- queue_output:
//	    queue_name: indexing-my-stream   # target queue (required)
//	    message_field: messages          # ctx batch key (default "messages")
//
// On queue push failure the processor returns an error so the consumer
// does not commit the offset and the batch is redelivered (at-least-once).
package queue_output

import (
	"fmt"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	log "infini.sh/framework/core/log"
)

const name = "queue_output"

type Config struct {
	QueueName    string `config:"queue_name"`
	MessageField string `config:"message_field"`
}

type Processor struct {
	cfg Config
}

func init() {
	pipeline.RegisterProcessorPlugin(name, New)
}

func New(c *config.Config) (pipeline.Processor, error) {
	cfg := Config{MessageField: "messages"}
	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of %s processor: %s", name, err)
	}
	if cfg.QueueName == "" {
		return nil, fmt.Errorf("%s processor requires queue_name", name)
	}
	return &Processor{cfg: cfg}, nil
}

func (p *Processor) Name() string { return name }

// Process appends every record of the context batch onto the target queue.
func (p *Processor) Process(c *pipeline.Context) error {
	v := c.Get(param.ParaKey(p.cfg.MessageField))
	msgs, ok := v.([]queue.Message)
	if !ok || len(msgs) == 0 {
		return nil
	}

	qConfig := queue.GetOrInitConfig(p.cfg.QueueName)
	pushed := 0
	for i := range msgs {
		if len(msgs[i].Data) == 0 {
			continue // dropped records (drop_event) are skipped
		}
		if err := queue.Push(qConfig, msgs[i].Data); err != nil {
			log.Errorf("%s: queue push failed after %d/%d records: %v", name, pushed, len(msgs), err)
			return fmt.Errorf("%s: queue push failed after %d/%d records: %w",
				name, pushed, len(msgs), err)
		}
		pushed++
	}
	return nil
}
