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
	"time"
)

type TaskScheduler interface {
	Schedule(task Task, options ...Option) (ScheduledTask, error)
	ScheduleWithCron(task Task, expression string, options ...Option) (ScheduledTask, error)
	ScheduleWithFixedDelay(task Task, delay time.Duration, options ...Option) (ScheduledTask, error)
	ScheduleAtFixedRate(task Task, period time.Duration, options ...Option) (ScheduledTask, error)
	IsShutdown() bool
	Shutdown() chan bool
}

type SimpleTaskScheduler struct {
	taskExecutor TaskExecutor
}

func NewSimpleTaskScheduler(executor TaskExecutor) *SimpleTaskScheduler {

	if executor == nil {
		executor = NewDefaultTaskExecutor()
	}

	scheduler := &SimpleTaskScheduler{
		taskExecutor: executor,
	}

	return scheduler
}

func NewDefaultTaskScheduler() TaskScheduler {
	return NewSimpleTaskScheduler(NewDefaultTaskExecutor())
}

func (scheduler *SimpleTaskScheduler) Schedule(task Task, options ...Option) (ScheduledTask, error) {
	schedulerTask, err := CreateSchedulerTask(task, options...)

	if err != nil {
		return nil, err
	}

	return scheduler.taskExecutor.Schedule(task, schedulerTask.GetInitialDelay())
}

func (scheduler *SimpleTaskScheduler) ScheduleWithCron(task Task, expression string, options ...Option) (ScheduledTask, error) {
	var schedulerTask *SchedulerTask
	var err error

	schedulerTask, err = CreateSchedulerTask(task, options...)

	if err != nil {
		return nil, err
	}

	var cronTrigger *CronTrigger
	cronTrigger, err = CreateCronTrigger(expression, schedulerTask.location)

	if err != nil {
		return nil, err
	}

	var triggerTask *TriggerTask
	triggerTask, err = CreateTriggerTask(schedulerTask.task, scheduler.taskExecutor, cronTrigger)

	if err != nil {
		return nil, err
	}

	return triggerTask.Schedule()
}

func (scheduler *SimpleTaskScheduler) ScheduleWithFixedDelay(task Task, delay time.Duration, options ...Option) (ScheduledTask, error) {
	schedulerTask, err := CreateSchedulerTask(task, options...)

	if err != nil {
		return nil, err
	}

	return scheduler.taskExecutor.ScheduleWithFixedDelay(schedulerTask.task, schedulerTask.GetInitialDelay(), delay)
}

func (scheduler *SimpleTaskScheduler) ScheduleAtFixedRate(task Task, period time.Duration, options ...Option) (ScheduledTask, error) {
	schedulerTask, err := CreateSchedulerTask(task, options...)

	if err != nil {
		return nil, err
	}

	return scheduler.taskExecutor.ScheduleAtFixedRate(schedulerTask.task, schedulerTask.GetInitialDelay(), period)
}

func (scheduler *SimpleTaskScheduler) IsShutdown() bool {
	return scheduler.taskExecutor.IsShutdown()
}

func (scheduler *SimpleTaskScheduler) Shutdown() chan bool {
	return scheduler.taskExecutor.Shutdown()
}
