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

type TriggerContext interface {
	LastCompletionTime() time.Time
	LastExecutionTime() time.Time
	LastTriggeredExecutionTime() time.Time
}

type SimpleTriggerContext struct {
	lastCompletionTime         time.Time
	lastExecutionTime          time.Time
	lastTriggeredExecutionTime time.Time
}

func NewSimpleTriggerContext() *SimpleTriggerContext {
	return &SimpleTriggerContext{}
}

func (ctx *SimpleTriggerContext) Update(lastCompletionTime time.Time, lastExecutionTime time.Time, lastTriggeredExecutionTime time.Time) {
	ctx.lastCompletionTime = lastCompletionTime
	ctx.lastExecutionTime = lastExecutionTime
	ctx.lastTriggeredExecutionTime = lastTriggeredExecutionTime
}

func (ctx *SimpleTriggerContext) LastCompletionTime() time.Time {
	return ctx.lastCompletionTime
}

func (ctx *SimpleTriggerContext) LastExecutionTime() time.Time {
	return ctx.lastExecutionTime
}

func (ctx *SimpleTriggerContext) LastTriggeredExecutionTime() time.Time {
	return ctx.lastTriggeredExecutionTime
}

type Trigger interface {
	NextExecutionTime(ctx TriggerContext) time.Time
}

type CronTrigger struct {
	cronExpression CronExpression
	location       *time.Location
}

func CreateCronTrigger(expression string, location *time.Location) (*CronTrigger, error) {
	cron, err := ParseCronExpression(expression)
	if err != nil {
		return nil, err
	}
	loc := cron.GetLocation()
	if loc == nil {
		if location != nil {
			loc = location
		}else{
			loc = time.Local
		}
	}

	trigger := &CronTrigger{
		cron,
		loc,
	}

	return trigger, nil
}

func (trigger *CronTrigger) NextExecutionTime(ctx TriggerContext) time.Time {
	now := time.Now()
	lastCompletion := ctx.LastCompletionTime()

	if !lastCompletion.IsZero() {

		lastExecution := ctx.LastTriggeredExecutionTime()

		if !lastExecution.IsZero() && now.Before(lastExecution) {
			now = lastExecution
		}

	}

	originalLocation := now.Location()
	next := trigger.cronExpression.NextTime(now.In(trigger.location))
	return next.In(originalLocation)
}
