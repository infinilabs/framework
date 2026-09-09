/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
)

// routeMarker stamps a fixed route onto the current record (test fixture
// for observing which if/then/else branch ran for each record).
type routeMarker struct {
	Route string `config:"route"`
}

func (r *routeMarker) Name() string { return "test_route_marker" }

func (r *routeMarker) Process(c *pipeline.Context) error {
	rec, ok := pipeline.CurrentRecord(c)
	if !ok {
		return nil
	}
	if rec.Fields == nil {
		rec.Fields = util.MapStr{}
	}
	rec.Fields["route"] = r.Route
	return nil
}

func init() {
	pipeline.RegisterProcessorPlugin("test_route_marker", func(c *config.Config) (pipeline.Processor, error) {
		m := &routeMarker{}
		if err := c.Unpack(m); err != nil {
			return nil, err
		}
		return m, nil
	})
}

// The motivating scenario of the record-context fix: an if/then/else inside
// a for_each sub-chain routes each record by its own fields. Before the fix
// every condition evaluated against the record-less pipeline parameters and
// never fired, so all records fell through to the same branch.
func TestForEach_IfThenElseRoutesEachRecord(t *testing.T) {
	const nginxMsg = `{"payload":{"file":{"path":"/var/log/nginx/access.log"},"message":"m1"},"timestamp":"2026-08-18T10:00:00Z"}`
	const redisMsg = `{"payload":{"file":{"path":"/var/log/redis/redis.log"},"message":"m2"},"timestamp":"2026-08-18T10:00:01Z"}`
	const junk = "not-an-envelope"

	c, err := config.NewConfigFrom(util.MapStr{
		"processor": []interface{}{
			util.MapStr{
				"if":   util.MapStr{"contains": util.MapStr{"file.path": "nginx"}},
				"then": util.MapStr{"test_route_marker": util.MapStr{"route": "nginx-pipeline"}},
				"else": util.MapStr{"test_route_marker": util.MapStr{"route": "default-pipeline"}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewForEachProcessor(c)
	if err != nil {
		t.Fatalf("NewForEachProcessor: %v", err)
	}
	fp := p.(*ForEachProcessor)

	msgs := []queue.Message{
		{Data: []byte(nginxMsg), Size: len(nginxMsg), Offset: queue.AcquireOffset(0, 1)},
		{Data: []byte(redisMsg), Size: len(redisMsg), Offset: queue.AcquireOffset(0, 2)},
		{Data: []byte(junk), Size: len(junk), Offset: queue.AcquireOffset(0, 3)},
	}
	ctx := &pipeline.Context{Context: t.Context()}
	ctx.Set(param.ParaKey("messages"), msgs)

	if err := fp.Process(ctx); err != nil {
		t.Fatal(err)
	}

	out, ok := ctx.Get(param.ParaKey("messages")).([]queue.Message)
	if !ok {
		t.Fatal("for_each must publish the batch back to the context")
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 messages in the batch, got %v", len(out))
	}

	nginxOut := string(out[0].Data)
	if !contains(nginxOut, "nginx-pipeline") || contains(nginxOut, "default-pipeline") {
		t.Fatalf("nginx record mis-routed: %s", nginxOut)
	}
	redisOut := string(out[1].Data)
	if !contains(redisOut, "default-pipeline") || contains(redisOut, "nginx-pipeline") {
		t.Fatalf("redis record mis-routed: %s", redisOut)
	}
	if string(out[2].Data) != junk {
		t.Fatalf("undecodable payload must pass through untouched, got: %s", out[2].Data)
	}
}

// Conditional routing must also work for records processed by a plain
// pipeline outside for_each — the then/else sub-chains still see the record
// bound by any other record-scoped host, and routing decisions must not leak
// from one record to the next.
func TestForEach_IfThenElseChainedConditions(t *testing.T) {
	const aMsg = `{"payload":{"log_level":"error","message":"boom"},"timestamp":"2026-08-18T10:00:00Z"}`
	const bMsg = `{"payload":{"log_level":"info","message":"hi"},"timestamp":"2026-08-18T10:00:01Z"}`

	c, err := config.NewConfigFrom(util.MapStr{
		"processor": []interface{}{
			util.MapStr{
				"if": util.MapStr{"equals": util.MapStr{"log_level": "error"}},
				"then": []interface{}{
					util.MapStr{"test_route_marker": util.MapStr{"route": "errors"}},
				},
				"else": []interface{}{
					util.MapStr{
						"if":   util.MapStr{"equals": util.MapStr{"log_level": "info"}},
						"then": util.MapStr{"test_route_marker": util.MapStr{"route": "infos"}},
						"else": util.MapStr{"test_route_marker": util.MapStr{"route": "others"}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewForEachProcessor(c)
	if err != nil {
		t.Fatalf("NewForEachProcessor: %v", err)
	}
	fp := p.(*ForEachProcessor)

	msgs := []queue.Message{
		{Data: []byte(aMsg), Size: len(aMsg), Offset: queue.AcquireOffset(0, 1)},
		{Data: []byte(bMsg), Size: len(bMsg), Offset: queue.AcquireOffset(0, 2)},
	}
	ctx := &pipeline.Context{Context: t.Context()}
	ctx.Set(param.ParaKey("messages"), msgs)

	if err := fp.Process(ctx); err != nil {
		t.Fatal(err)
	}

	out, _ := ctx.Get(param.ParaKey("messages")).([]queue.Message)
	if len(out) != 2 {
		t.Fatalf("expected 2 messages in the batch, got %v", len(out))
	}
	if errOut := string(out[0].Data); !contains(errOut, "errors") || contains(errOut, "infos") {
		t.Fatalf("error record mis-routed: %s", errOut)
	}
	if infoOut := string(out[1].Data); !contains(infoOut, "infos") || contains(infoOut, "errors") {
		t.Fatalf("info record mis-routed: %s", infoOut)
	}
}
