package consumer

import (
	"testing"

	"infini.sh/framework/core/config"
)

// The documented standalone bridge (consumer{codec, output_lane} with no
// inner chain) must construct and release — it used to be rejected by
// the non-empty-chain validation while bridgeToLane had no call site
// (PR #433 review).
func TestOutputLaneBridgeWithoutInnerChain(t *testing.T) {
	cfg, err := config.NewConfigFrom(map[string]interface{}{"output_lane": "pr433-bridge", "codec": "otel"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("documented consumer{codec,output_lane} bridge rejected without an inner processor: %v", err)
	}
	if p.(*QueueConsumerProcessor).config.OutputLane != "pr433-bridge" {
		t.Fatalf("bridge lane not configured: %+v", p.(*QueueConsumerProcessor).config)
	}
	if err := p.(*QueueConsumerProcessor).Release(); err != nil {
		t.Fatal(err)
	}
}

// Bridging AND an inner chain is ambiguous — the silent
// output_lane-wins behavior must not pass validation.
func TestConsumerRejectsBridgePlusInnerChain(t *testing.T) {
	cfg, err := config.NewConfigFrom(map[string]interface{}{
		"output_lane": "pr433-bridge",
		"codec":       "otel",
		"processor":   []interface{}{map[string]interface{}{"output": map[string]interface{}{"queue": "x"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(cfg); err == nil {
		t.Fatal("consumer accepted both output_lane and an inner processor chain")
	}
}
