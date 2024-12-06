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

package pipeline

import (
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/conditions"
	"github.com/rubyniu105/framework/core/config"
	"github.com/rubyniu105/framework/core/global"
	"strings"
)

// NewConditional returns a constructor suitable for registering when conditionals as a plugin.
func NewConditional(
	ruleFactory ProcessorConstructor,
) ProcessorConstructor {
	return func(cfg *config.Config) (Processor, error) {
		rule, err := ruleFactory(cfg)
		if err != nil {
			return nil, err
		}
		return addCondition(cfg, rule)
	}
}

// NewConditionList takes a slice of Config objects and turns them into real Condition objects.
func NewConditionList(config []conditions.Config) ([]conditions.Condition, error) {
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

// WhenProcessor is a tuple of condition plus a Processor.
type WhenProcessor struct {
	condition conditions.Condition
	p         Processor
}

// NewConditionRule returns a processor that will execute the provided processor if the condition is true.
func NewConditionRule(
	config conditions.Config,
	p Processor,
) (Processor, error) {
	cond, err := conditions.NewCondition(&config)
	if err != nil {
		return nil, errors.Unwrap(err)
	}

	if cond == nil {
		return p, nil
	}
	return &WhenProcessor{cond, p}, nil
}

// Run executes this WhenProcessor.
func (r WhenProcessor) Process(ctx *Context) error {
	if !ctx.ShouldContinue() {
		if global.Env().IsDebug {
			log.Debugf("filter [%v] not continued", r.Name())
		}
		ctx.AddFlowProcess(r.Name() + "-skipped")
		return nil
	}

	if !(r.condition).Check(ctx) {
		ctx.AddFlowProcess(r.p.Name() + "-skipped")
		return nil
	}
	ctx.AddFlowProcess(r.p.Name())
	r.p.Process(ctx)
	return nil
}

func (r WhenProcessor) Name() string {
	return "when"
}

func (r *WhenProcessor) String() string {
	return fmt.Sprintf("%v, condition=%v", r.p.Name(), r.condition.String())
}

func addCondition(
	cfg *config.Config,
	p Processor,
) (Processor, error) {
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

	return NewConditionRule(condConfig, p)
}

type ifThenElseConfig struct {
	Cond conditions.Config `config:"if"   validate:"required"`
	Then *config.Config    `config:"then" validate:"required"`
	Else *config.Config    `config:"else"`
}

// IfThenElseProcessor executes one set of processors (then) if the condition is
// true and another set of processors (else) if the condition is false.
type IfThenElseProcessor struct {
	cond conditions.Condition
	then *Processors
	els  *Processors
}

// NewIfElseThenProcessor construct a new IfThenElseProcessor.
func NewIfElseThenProcessor(cfg *config.Config) (*IfThenElseProcessor, error) {
	var tempConfig ifThenElseConfig
	if err := cfg.Unpack(&tempConfig); err != nil {
		return nil, err
	}

	cond, err := conditions.NewCondition(&tempConfig.Cond)
	if err != nil {
		return nil, err
	}

	newProcessors := func(c *config.Config) (*Processors, error) {
		if c == nil {
			return nil, nil
		}
		if !c.IsArray() {
			return NewPipeline([]*config.Config{c})
		}

		var pc []*config.Config
		if err := c.Unpack(&pc); err != nil {
			return nil, err
		}
		return NewPipeline(pc)
	}

	var ifProcessors, elseProcessors *Processors
	if ifProcessors, err = newProcessors(tempConfig.Then); err != nil {
		return nil, err
	}
	if elseProcessors, err = newProcessors(tempConfig.Else); err != nil {
		return nil, err
	}

	return &IfThenElseProcessor{cond, ifProcessors, elseProcessors}, nil
}

// Run checks the if condition and executes the processors attached to the
// then statement or the else statement based on the condition.
func (p IfThenElseProcessor) Process(ctx *Context) error {
	if !ctx.ShouldContinue() {
		if global.Env().IsDebug {
			log.Debugf("filter [%v] not continued", p.Name())
		}
		ctx.AddFlowProcess("skipped")
		return nil
	}

	if p.cond.Check(ctx) {
		if global.Env().IsDebug {
			log.Trace("if -> then branch")
		}
		ctx.AddFlowProcess("then")
		p.then.Process(ctx)
	} else if p.els != nil {
		if global.Env().IsDebug {
			log.Trace("if -> else branch")
		}
		ctx.AddFlowProcess("else")
		p.els.Process(ctx)
	}
	return nil
}

func (p IfThenElseProcessor) Name() string {
	return "if"
}

func (p *IfThenElseProcessor) String() string {
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
