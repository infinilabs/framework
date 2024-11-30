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

// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package pipeline

import (
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/conditions"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/global"
	"infini.sh/framework/lib/fasthttp"
	"strings"
)

// NewFilterConditional returns a constructor suitable for registering when conditionals as a plugin.
func NewFilterConditional(
	ruleFactory FilterConstructor,
) FilterConstructor {
	return func(cfg *config.Config) (Filter, error) {
		rule, err := ruleFactory(cfg)
		if err != nil {
			return nil, err
		}
		return addFilterCondition(cfg, rule)
	}
}

// NewFilterConditionList takes a slice of Config objects and turns them into real Condition objects.
func NewFilterConditionList(config []conditions.Config) ([]conditions.Condition, error) {
	out := make([]conditions.Condition, len(config))
	for i, condConfig := range config {
		cond, err := conditions.NewCondition(&condConfig)
		if err != nil {
			return nil, err
		}

		out[i] = cond
	}
	return out, nil
}

// WhenFilter is a tuple of condition plus a Processor.
type WhenFilter struct {
	condition conditions.Condition
	p         Filter
}

// NewFilterConditionRule returns a processor that will execute the provided processor if the condition is true.
func NewFilterConditionRule(
	config conditions.Config,
	p Filter,
) (Filter, error) {
	cond, err := conditions.NewCondition(&config)
	if err != nil {
		return nil, errors.Unwrap(err)
	}

	if cond == nil {
		return p, nil
	}
	return &WhenFilter{cond, p}, nil
}

// Run executes this WhenFilter.
func (r WhenFilter) Filter(ctx *fasthttp.RequestCtx) {

	if !ctx.ShouldContinue() {
		if global.Env().IsDebug {
			log.Debugf("filter [%v] not continued", r.Name())
		}
		ctx.AddFlowProcess(r.Name() + "-skipped")
		return
	}

	if !(r.condition).Check(ctx) {
		ctx.AddFlowProcess(r.p.Name() + "-skipped")
		return
	}

	ctx.AddFlowProcess(r.p.Name())
	r.p.Filter(ctx)
}

func (r WhenFilter) Name() string {
	return "when"
}

func (r *WhenFilter) String() string {
	return fmt.Sprintf("%v, condition=%v", r.p.Name(), r.condition.String())
}

func addFilterCondition(
	cfg *config.Config,
	p Filter,
) (Filter, error) {
	if !cfg.HasField("when") {
		return p, nil
	}
	sub, err := cfg.Child("when", -1)
	if err != nil {
		return nil, err
	}

	condConfig := conditions.Config{}
	if err := sub.Unpack(&condConfig); err != nil {
		return nil, err
	}

	return NewFilterConditionRule(condConfig, p)
}

// IfThenElseFilter executes one set of processors (then) if the condition is
// true and another set of processors (else) if the condition is false.
type IfThenElseFilter struct {
	cond conditions.Condition
	then *Filters
	els  *Filters
}

// NewIfElseThenFilter construct a new IfThenElseFilter.
func NewIfElseThenFilter(cfg *config.Config) (*IfThenElseFilter, error) {
	var tempConfig ifThenElseConfig
	if err := cfg.Unpack(&tempConfig); err != nil {
		return nil, err
	}

	cond, err := conditions.NewCondition(&tempConfig.Cond)
	if err != nil {
		return nil, err
	}

	newProcessors := func(c *config.Config) (*Filters, error) {
		if c == nil {
			return nil, nil
		}
		if !c.IsArray() {
			return NewFilter([]*config.Config{c})
		}

		var pc []*config.Config
		if err := c.Unpack(&pc); err != nil {
			return nil, err
		}
		return NewFilter(pc)
	}

	var ifProcessors, elseProcessors *Filters
	if ifProcessors, err = newProcessors(tempConfig.Then); err != nil {
		return nil, err
	}
	if elseProcessors, err = newProcessors(tempConfig.Else); err != nil {
		return nil, err
	}

	return &IfThenElseFilter{cond, ifProcessors, elseProcessors}, nil
}

// Run checks the if condition and executes the processors attached to the
// then statement or the else statement based on the condition.
func (p IfThenElseFilter) Filter(ctx *fasthttp.RequestCtx) {
	if !ctx.ShouldContinue() {
		if global.Env().IsDebug {
			log.Debugf("filter [%v] not continued", p.Name())
		}
		ctx.AddFlowProcess("skipped")
		return
	}

	if p.cond.Check(ctx) {
		if global.Env().IsDebug {
			log.Trace("if -> then branch")
		}
		ctx.AddFlowProcess("then")
		p.then.Filter(ctx)
	} else if p.els != nil {
		if global.Env().IsDebug {
			log.Trace("if -> else branch")
		}
		ctx.AddFlowProcess("else")
		p.els.Filter(ctx)
	}
}

func (p IfThenElseFilter) Name() string {
	return "if"
}

func (p *IfThenElseFilter) String() string {
	var sb strings.Builder
	sb.WriteString("if ")
	sb.WriteString(p.cond.String())
	sb.WriteString(" then ")
	sb.WriteString(p.then.Name())
	if p.els != nil {
		sb.WriteString(" else ")
		sb.WriteString(p.els.Name())
	}
	return sb.String()
}
