// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello#infini.ltd
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

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

// Package export provides the "otlp_export" pipeline processor: the agent
// side of the OTLP/gRPC transport. It drains a batch of queue messages
// (the otel JSON envelope produced upstream), renders them as a single
// OTLP ExportLogsServiceRequest and ships it to a collector endpoint
// (typically the INFINI gateway's OTLP intake on :4317).
//
// On failure the processor returns an error, which keeps the batch
// unacknowledged so the consumer redelivers it once the endpoint is
// reachable again — the local disk queue acts as the durable buffer.
//
// Configuration:
//
//	- otlp_export:
//	    endpoint: gateway:4317     # OTLP/gRPC collector address
//	    insecure: true             # plain text (no TLS)
//	    timeout: 10s
//	    headers:                   # static metadata
//	      x-infini-agent: "$[[env.NODE_ID]]"
//	    message_field: messages    # where the consumer stored the batch
package export

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/otel"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	otlpcodec "infini.sh/framework/plugins/otlp"
)

const name = "otlp_export"

func init() {
	pipeline.RegisterProcessorPlugin(name, New)
}

// Config of the otlp_export processor.
type Config struct {
	Endpoint     string            `config:"endpoint"`
	Insecure     bool              `config:"insecure"`
	Timeout      string            `config:"timeout"`
	Headers      map[string]string `config:"headers"`
	MessageField string            `config:"message_field"`
}

// Processor implements pipeline.Processor.
type Processor struct {
	cfg Config

	timeout     time.Duration
	mu          sync.Mutex
	conn        *grpc.ClientConn
	client      logsv1.LogsServiceClient
	endpoint    string
	sentRecords uint64
	failedRPCs  uint64
}

// New builds the processor from configuration.
func New(c *config.Config) (pipeline.Processor, error) {
	cfg := Config{Insecure: true, Timeout: "10s", MessageField: "messages"}
	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of %s processor: %s", name, err)
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("%s processor requires an endpoint", name)
	}
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil || timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Processor{cfg: cfg, timeout: timeout}, nil
}

// Name implements pipeline.Processor.
func (p *Processor) Name() string { return name }

// Process implements pipeline.Processor.
func (p *Processor) Process(ctx *pipeline.Context) error {
	v := ctx.Get(param.ParaKey(p.cfg.MessageField))
	msgs, ok := v.([]queue.Message)
	if !ok || len(msgs) == 0 {
		return nil
	}

	events := make([]*event.Event, 0, len(msgs))
	for i := range msgs {
		if len(msgs[i].Data) == 0 {
			continue
		}
		e, err := otel.DecodeEnvelope(msgs[i].Data)
		if err != nil {
			log.Warnf("%s: skipping undecodable message at offset %v: %v", name, msgs[i].Offset, err)
			continue
		}
		events = append(events, e)
	}
	if len(events) == 0 {
		return nil
	}

	req := otlpcodec.BuildExportRequest(events)
	if err := p.export(ctx, req); err != nil {
		p.mu.Lock()
		p.failedRPCs++
		p.mu.Unlock()
		// keep the batch unacknowledged: the queue redelivers on retry
		return fmt.Errorf("%s: failed to export %d records: %v", name, len(events), err)
	}
	p.mu.Lock()
	p.sentRecords += uint64(len(events))
	p.mu.Unlock()
	return nil
}

func (p *Processor) export(ctx *pipeline.Context, req *logsv1.ExportLogsServiceRequest) error {
	client, err := p.getClient()
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	if len(p.cfg.Headers) > 0 {
		md := metadata.New(p.cfg.Headers)
		callCtx = metadata.NewOutgoingContext(callCtx, md)
	}
	_, err = client.Export(callCtx, req)
	return err
}

// getClient lazily dials the collector and caches the connection,
// re-dialing when the configured endpoint changed.
func (p *Processor) getClient() (logsv1.LogsServiceClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil && p.endpoint == p.cfg.Endpoint {
		return p.client, nil
	}
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}

	opts := []grpc.DialOption{}
	if p.cfg.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.NewClient(p.cfg.Endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("%s: dial %s: %v", name, p.cfg.Endpoint, err)
	}
	p.conn = conn
	p.endpoint = p.cfg.Endpoint
	p.client = logsv1.NewLogsServiceClient(conn)
	return p.client, nil
}

// Close implements pipeline.Closer.
func (p *Processor) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		err := p.conn.Close()
		p.conn = nil
		return err
	}
	return nil
}
