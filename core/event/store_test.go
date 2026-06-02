package event

import (
	"errors"
	"testing"

	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
)

func TestNormalizeEventLabelsFlattensDSLWrappers(t *testing.T) {
	item := &Event{
		Metadata: EventMetadata{
			Labels: util.MapStr{
				"cluster_id": util.MapStr{
					"terms": "infini_default_system_cluster",
				},
				"cluster_uuid": map[string]interface{}{
					"value": "cluster-uuid",
				},
				"roles": []interface{}{
					util.MapStr{"value": "data"},
					map[string]interface{}{"terms": "ingest"},
				},
			},
		},
	}

	normalizeEventLabels(item)

	if got := item.Metadata.Labels["cluster_id"]; got != "infini_default_system_cluster" {
		t.Fatalf("expected flattened cluster_id label, got %#v", got)
	}
	if got := item.Metadata.Labels["cluster_uuid"]; got != "cluster-uuid" {
		t.Fatalf("expected flattened cluster_uuid label, got %#v", got)
	}

	roles, ok := item.Metadata.Labels["roles"].([]interface{})
	if !ok {
		t.Fatalf("expected roles to remain a slice, got %#v", item.Metadata.Labels["roles"])
	}
	if len(roles) != 2 || roles[0] != "data" || roles[1] != "ingest" {
		t.Fatalf("expected flattened roles entries, got %#v", roles)
	}
}

func TestSaveLogReturnsQueueError(t *testing.T) {
	originalPush := pushQueueMessage
	originalGetOrInitQueueConfig := getOrInitQueueConfig
	originalMeta := meta
	t.Cleanup(func() {
		pushQueueMessage = originalPush
		getOrInitQueueConfig = originalGetOrInitQueueConfig
		meta = originalMeta
	})

	pushQueueMessage = func(_ *queue.QueueConfig, _ []byte) error {
		return errors.New("readonly")
	}
	getOrInitQueueConfig = func(_ string) *queue.QueueConfig {
		return &queue.QueueConfig{}
	}
	meta = &AgentMeta{LoggingQueueName: "logging"}

	err := SaveLog(&Event{
		Metadata: EventMetadata{
			Category: "task",
			Name:     "logging",
		},
	})
	if err == nil || err.Error() != "readonly" {
		t.Fatalf("expected readonly error, got %v", err)
	}
}
