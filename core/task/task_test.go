package task

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"infini.sh/framework/core/task/chrono"
)

func TestInitialDelayOption(t *testing.T) {
	before := time.Now().Truncate(time.Second)
	schedulerTask, err := chrono.CreateSchedulerTask(func(ctx context.Context) {}, initialDelayOption(90*time.Second))
	assert.NoError(t, err)

	delay := schedulerTask.GetInitialDelay()
	// Second-granularity truncation may shave up to one second.
	assert.GreaterOrEqual(t, delay, 89*time.Second)
	assert.LessOrEqual(t, delay, time.Since(before)+90*time.Second)
}

// An interval task with InitialDelay must not fire at the first tick: with
// a 3s delay (chrono truncates to seconds, worst case ~2s effective) no run
// may happen before 1.2s, and one must have happened by 4.5s.
func TestScheduleTaskInitialDelayDefersFirstRun(t *testing.T) {
	var runs atomic.Int32
	taskID := RegisterScheduleTask(ScheduleTask{
		ID:           "test_initial_delay",
		Description:  "interval task with initial delay",
		Type:         Interval,
		Interval:     "200ms",
		InitialDelay: "3s",
		Task: func(ctx context.Context) {
			runs.Add(1)
		},
	})
	defer StopTask(taskID)

	RunTasks()

	time.Sleep(1200 * time.Millisecond)
	assert.Zero(t, runs.Load(), "first run must be delayed by the initial delay")

	deadline := time.Now().Add(3500 * time.Millisecond)
	for time.Now().Before(deadline) && runs.Load() == 0 {
		time.Sleep(100 * time.Millisecond)
	}
	assert.GreaterOrEqual(t, runs.Load(), int32(1), "first run must fire after the initial delay")
}
