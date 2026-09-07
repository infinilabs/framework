package pipeline

import (
	"testing"
)

func TestEqualsNestedChange(t *testing.T) {
	old := PipelineConfigV2{
		Name: "p1",
		Processors: []map[string]interface{}{
			{"logs_processor": map[string]interface{}{
				"logs_path": "/x",
				"ship_config": map[string]interface{}{
					"endpoints": []interface{}{"192.168.43.62:4317"},
				},
			}},
		},
	}
	neu := PipelineConfigV2{
		Name: "p1",
		Processors: []map[string]interface{}{
			{"logs_processor": map[string]interface{}{
				"logs_path": "/x",
				"ship_config": map[string]interface{}{
					"endpoints": []interface{}{"127.0.0.1:4317"},
				},
			}},
		},
	}
	if old.Equals(neu) {
		t.Fatal("nested endpoint change NOT detected by Equals")
	}
	t.Log("Equals detects nested change correctly")
}
