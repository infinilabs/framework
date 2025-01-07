// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
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

package chrono

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewSchedulerTask(t *testing.T) {
	_, err := CreateSchedulerTask(nil)
	assert.Error(t, err)
}

func TestNewSchedulerTask_WithLocation(t *testing.T) {
	_, err := CreateSchedulerTask(func(ctx context.Context) {

	}, WithLocation("Europe/Istanbul"))
	assert.Nil(t, err)
}

func TestNewSchedulerTask_WithInvalidLocation(t *testing.T) {
	_, err := CreateSchedulerTask(func(ctx context.Context) {

	}, WithLocation("Europe"))
	assert.Error(t, err)
}

func TestNewScheduledRunnableTask(t *testing.T) {
	task, _ := CreateScheduledRunnableTask(0, func(ctx context.Context) {

	}, time.Now(), -1, true)

	assert.Equal(t, task.period, 0*time.Second)

	_, err := CreateScheduledRunnableTask(0, nil, time.Now(), -1, true)
	assert.Error(t, err)
}

func TestNewTriggerTaskWithTimezone(t *testing.T) {
	trigger, err := CreateCronTrigger("CRON_TZ=America/New_York 0 9 18 * * 1", time.Local)
	assert.Nil(t, err)
	loc, _ := time.LoadLocation("America/New_York")
	assert.Equal(t, loc, trigger.location)
	ctx := NewSimpleTriggerContext()
	tm := trigger.NextExecutionTime(ctx)
	assert.Equal(t, 7, tm.Hour())
}

func TestNewTriggerTask(t *testing.T) {
	trigger, err := CreateCronTrigger("* * * * * *", time.Local)
	assert.Nil(t, err)

	_, err = CreateTriggerTask(nil, NewDefaultTaskExecutor(), trigger)
	assert.Error(t, err)

	_, err = CreateTriggerTask(func(ctx context.Context) {

	}, nil, trigger)
	assert.Error(t, err)

	_, err = CreateTriggerTask(func(ctx context.Context) {

	}, NewDefaultTaskExecutor(), nil)
}

type zeroTrigger struct {
}

func (trigger *zeroTrigger) NextExecutionTime(ctx TriggerContext) time.Time {
	return time.Time{}
}

func TestTriggerTask_Schedule(t *testing.T) {
	task, _ := CreateTriggerTask(func(ctx context.Context) {}, NewDefaultTaskExecutor(), &zeroTrigger{})
	_, err := task.Schedule()
	assert.NotNil(t, err)
}

type scheduledExecutorMock struct {
	mock.Mock
}

func (executor scheduledExecutorMock) Schedule(task Task, delay time.Duration) (ScheduledTask, error) {
	result := executor.Called(task, delay)
	return result.Get(0).(ScheduledTask), result.Error(1)
}

func (executor scheduledExecutorMock) ScheduleWithFixedDelay(task Task, initialDelay time.Duration, delay time.Duration) (ScheduledTask, error) {
	result := executor.Called(task, initialDelay, delay)
	return result.Get(0).(ScheduledTask), result.Error(1)
}

func (executor scheduledExecutorMock) ScheduleAtFixedRate(task Task, initialDelay time.Duration, period time.Duration) (ScheduledTask, error) {
	result := executor.Called(task, initialDelay, period)
	return result.Get(0).(ScheduledTask), result.Error(1)
}

func (executor scheduledExecutorMock) IsShutdown() bool {
	result := executor.Called()
	return result.Bool(0)
}

func (executor scheduledExecutorMock) Shutdown() chan bool {
	result := executor.Called()
	return result.Get(0).(chan bool)
}

func TestTriggerTask_ScheduleWithError(t *testing.T) {
	scheduledExecutorMock := &scheduledExecutorMock{}

	scheduledExecutorMock.On("Schedule", mock.AnythingOfType("Task"), mock.AnythingOfType("time.Duration")).
		Return((*ScheduledRunnableTask)(nil), errors.New("test error"))

	trigger, err := CreateCronTrigger("* * * * * *", time.Local)
	assert.Nil(t, err)

	task, _ := CreateTriggerTask(func(ctx context.Context) {}, scheduledExecutorMock, trigger)
	_, err = task.Schedule()

	assert.NotNil(t, err)
}
