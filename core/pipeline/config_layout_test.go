/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infini.ltd */

package pipeline

import (
	"encoding/json"
	"strings"
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/util"
)

// TestPipelineConfigV2CanvasLayoutRoundTrip (#109): the orchestration
// UI's node positions ride with the config through config unpacking and
// JSON serialization — presentation state only, but it must survive both
// directions untouched.
//
// Keys follow the "n<index>" convention documented on the field: ucfg
// reads purely numeric map keys as array indices and drops them, so "0"
// would unpack to nothing.
func TestPipelineConfigV2CanvasLayoutRoundTrip(t *testing.T) {
	cfgMap := util.MapStr{
		"name": "layout-probe",
		"processor": []interface{}{
			util.MapStr{"echo": util.MapStr{"message": "hi"}},
		},
		"canvas_layout": util.MapStr{
			"n0": util.MapStr{"x": 120.5, "y": 40},
			"n1": util.MapStr{"x": 340, "y": 40},
		},
	}
	c, err := config.NewConfigFrom(cfgMap)
	if err != nil {
		t.Fatal(err)
	}
	var cfg PipelineConfigV2
	if err := c.Unpack(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.CanvasLayout == nil {
		t.Fatal("canvas_layout must unpack onto the config")
	}
	n0, _ := cfg.CanvasLayout["n0"].(map[string]interface{})
	if n0 == nil || n0["x"] != 120.5 {
		t.Fatalf("layout node n0 lost: %v", cfg.CanvasLayout["n0"])
	}

	// JSON round-trip (API + persistence shape)
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]interface{}
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	layout, _ := back["canvas_layout"].(map[string]interface{})
	if len(layout) != 2 {
		t.Fatalf("canvas_layout must survive JSON: %v", back["canvas_layout"])
	}

	// absent stays absent (omitempty) — legacy configs are unchanged
	var legacy PipelineConfigV2
	lb, _ := json.Marshal(legacy)
	if strings.Contains(string(lb), "canvas_layout") {
		t.Fatalf("empty layout must not serialize: %s", lb)
	}
}
