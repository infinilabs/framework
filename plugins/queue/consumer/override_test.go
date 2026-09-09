package consumer

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"infini.sh/framework/core/queue"
)

// The processor folds its `consumer` block into the registry config via
// applyProcessorConsumerOverrides — on a CLONE. These tests pin both the
// field coverage (including the three fields that were previously missing)
// and the clone independence.
func TestApplyProcessorConsumerOverrides_FullCoverage(t *testing.T) {
	base := &queue.ConsumerConfig{
		FetchMinBytes:           1,
		FetchMaxBytes:           100,
		FetchMaxMessages:        10,
		FetchMaxWaitMs:          1000,
		ConsumeTimeoutInSeconds: 30,
		EOFMaxRetryTimes:        3,
		EOFRetryDelayInMs:       100,
		ClientExpiredInSeconds:  60,
		AutoResetOffset:         "earliest",
	}
	src := &queue.ConsumerConfig{
		FetchMinBytes:           2,
		FetchMaxBytes:           200,
		FetchMaxMessages:        20,
		FetchMaxWaitMs:          2000,
		ConsumeTimeoutInSeconds: 45,
		EOFMaxRetryTimes:        7,
		EOFRetryDelayInMs:       200,
		ClientExpiredInSeconds:  90,
		AutoResetOffset:         "latest",
	}
	applyProcessorConsumerOverrides(base, src)
	assert.Equal(t, 2, base.FetchMinBytes)
	assert.Equal(t, 200, base.FetchMaxBytes)
	assert.Equal(t, 20, base.FetchMaxMessages)
	assert.Equal(t, int64(2000), base.FetchMaxWaitMs)
	assert.Equal(t, 45, base.ConsumeTimeoutInSeconds)
	assert.Equal(t, 7, base.EOFMaxRetryTimes, "eof_max_retry_times must be overridable")
	assert.Equal(t, int64(200), base.EOFRetryDelayInMs)
	assert.Equal(t, int64(90), base.ClientExpiredInSeconds, "client_expired_in_seconds must be overridable")
	assert.Equal(t, "latest", base.AutoResetOffset, "auto_reset_offset must be overridable")
}

// Zero values on the processor side mean "not configured" and must keep the
// registry value — including the newly covered fields.
func TestApplyProcessorConsumerOverrides_ZeroValuesKeepBase(t *testing.T) {
	base := &queue.ConsumerConfig{
		FetchMinBytes:           1,
		FetchMaxBytes:           100,
		FetchMaxMessages:        10,
		FetchMaxWaitMs:          1000,
		ConsumeTimeoutInSeconds: 30,
		EOFMaxRetryTimes:        3,
		EOFRetryDelayInMs:       100,
		ClientExpiredInSeconds:  60,
		AutoResetOffset:         "earliest",
	}
	applyProcessorConsumerOverrides(base, &queue.ConsumerConfig{})
	assert.Equal(t, 1, base.FetchMinBytes)
	assert.Equal(t, 100, base.FetchMaxBytes)
	assert.Equal(t, 10, base.FetchMaxMessages)
	assert.Equal(t, int64(1000), base.FetchMaxWaitMs)
	assert.Equal(t, 30, base.ConsumeTimeoutInSeconds)
	assert.Equal(t, 3, base.EOFMaxRetryTimes)
	assert.Equal(t, int64(100), base.EOFRetryDelayInMs)
	assert.Equal(t, int64(60), base.ClientExpiredInSeconds)
	assert.Equal(t, "earliest", base.AutoResetOffset)

	// nil source is a no-op
	applyProcessorConsumerOverrides(base, nil)
	assert.Equal(t, 10, base.FetchMaxMessages)
}

// Clone independence: overriding the worker copy must never write back into
// the shared registry object — two processors with different fetch settings
// consuming the same queue+group previously clobbered each other.
func TestConsumerConfigCloneIndependence(t *testing.T) {
	shared := queue.NewConsumerConfig("queue-1", "group-1", "c1")
	shared.FetchMaxMessages = 500
	shared.FetchMaxBytes = 20 * 1024 * 1024
	shared.AutoResetOffset = "earliest"

	worker := shared.Clone()
	applyProcessorConsumerOverrides(worker, &queue.ConsumerConfig{
		FetchMaxMessages: 50,
		FetchMaxBytes:    1024,
		AutoResetOffset:  "latest",
	})

	assert.Equal(t, 50, worker.FetchMaxMessages)
	assert.Equal(t, 1024, worker.FetchMaxBytes)
	assert.Equal(t, "latest", worker.AutoResetOffset)

	assert.Equal(t, 500, shared.FetchMaxMessages, "registry copy must stay untouched")
	assert.Equal(t, 20*1024*1024, shared.FetchMaxBytes, "registry copy must stay untouched")
	assert.Equal(t, "earliest", shared.AutoResetOffset, "registry copy must stay untouched")

	// identity fields survive the clone (offset/commit paths key on them)
	assert.Equal(t, shared.ID, worker.ID)
	assert.Equal(t, shared.Group, worker.Group)
	assert.Equal(t, shared.Name, worker.Name)
	assert.Equal(t, shared.Key(), worker.Key())
}
