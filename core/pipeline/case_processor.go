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
	"reflect"
	"strings"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/conditions"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

// caseBranchConfig is one arm of a case block as written in YAML: an
// optional enumerated value set (matched against the case block's field),
// an optional full condition (`when`, any operator the if/then/else block
// accepts), and the chain to run on match. At least one of values/when is
// required; specifying both means AND.
type caseBranchConfig struct {
	Values []interface{}     `config:"values"`
	When   conditions.Config `config:"when"`
	Then   *config.Config    `config:"then"`
}

type caseConfig struct {
	Field    string             `config:"field"`
	Branches []caseBranchConfig `config:"branches"`
	Default  *config.Config     `config:"default"`
}

// caseBranch is the compiled form of one arm.
type caseBranch struct {
	values    []interface{}
	hasValues bool
	cond      conditions.Condition // nil = values-only arm
	then      *Processors
}

// CaseProcessor is the multi-way dispatch the if/then/else ladder becomes
// unwieldy for: ordered branches, FIRST match wins (exactly one arm runs),
// an optional default chain when nothing matches.
//
//	- case:
//	    field: level
//	    branches:
//	      - values: [error, fatal]
//	        then: [ ... ]
//	      - when:
//	          prefix: {level: wa}
//	        then: [ ... ]
//	    default: [ ... ]
type CaseProcessor struct {
	field    string // "" = condition-only arms
	branches []caseBranch
	def      *Processors
}

// NewCaseProcessor compiles a case step config at the default batch scope.
func NewCaseProcessor(cfg *config.Config) (*CaseProcessor, error) {
	return NewCaseProcessorWithScope(cfg, ScopeInfo{Kind: ScopeBatch})
}

// NewCaseProcessorWithScope compiles the arm chains under the ENCLOSING
// scope — the same inheritance if/then/else applies: inside a per-record
// sub-chain (for_each, lane_consumer) record-scope contracts keep
// applying and batch_only processors stay rejected in nested chains.
func NewCaseProcessorWithScope(cfg *config.Config, scope ScopeInfo) (*CaseProcessor, error) {
	sub, err := cfg.Child("case", -1)
	if err != nil {
		return nil, err
	}

	var c caseConfig
	if err := sub.Unpack(&c); err != nil {
		return nil, err
	}
	if len(c.Branches) == 0 {
		return nil, errors.New("case requires at least one branch")
	}

	newChain := func(cc *config.Config) (*Processors, error) {
		if cc == nil {
			return nil, nil
		}
		if !cc.IsArray() {
			return newPipelineWithScope([]*config.Config{cc}, scope, false)
		}
		var pc []*config.Config
		if err := cc.Unpack(&pc); err != nil {
			return nil, err
		}
		return newPipelineWithScope(pc, scope, false)
	}

	p := &CaseProcessor{field: c.Field}
	for i, b := range c.Branches {
		hasValues := len(b.Values) > 0
		hasWhen := !reflect.DeepEqual(b.When, conditions.Config{})
		if !hasValues && !hasWhen {
			return nil, fmt.Errorf("case branch %d requires `values` or `when`", i+1)
		}
		if hasValues && c.Field == "" {
			return nil, fmt.Errorf("case branch %d uses `values` but the case block declares no `field`", i+1)
		}
		arm := caseBranch{values: b.Values, hasValues: hasValues}
		if hasWhen {
			cond, err := conditions.NewCondition(&b.When)
			if err != nil {
				return nil, fmt.Errorf("case branch %d: %v", i+1, err)
			}
			arm.cond = cond
		}
		then, err := newChain(b.Then)
		if err != nil {
			return nil, fmt.Errorf("case branch %d: %v", i+1, err)
		}
		if then == nil {
			return nil, fmt.Errorf("case branch %d requires a `then` chain", i+1)
		}
		arm.then = then
		p.branches = append(p.branches, arm)
	}

	def, err := newChain(c.Default)
	if err != nil {
		return nil, fmt.Errorf("case default: %v", err)
	}
	p.def = def
	return p, nil
}

// Process routes to the first matching arm. Value resolution matches
// if/then/else: inside a per-record sub-chain arms see the current
// record's fields; pipeline-level usage reads the context.
func (p *CaseProcessor) Process(ctx *Context) error {
	if !ctx.ShouldContinue() {
		if global.Env().IsDebug {
			log.Debugf("filter [%v] not continued", p.Name())
		}
		ctx.AddFlowProcess(p.Name() + "-skipped")
		return nil
	}

	values := conditions.ValuesMap(ctx)
	if rec, ok := CurrentRecord(ctx); ok {
		values = rec
	}

	for i := range p.branches {
		arm := &p.branches[i]
		if arm.hasValues && !p.matchValues(values, arm.values) {
			continue
		}
		if arm.cond != nil && !arm.cond.Check(values) {
			continue
		}
		if global.Env().IsDebug {
			log.Tracef("case -> branch %d", i+1)
		}
		ctx.AddFlowProcess(fmt.Sprintf("%s:%d", p.Name(), i+1))
		return arm.then.Process(ctx)
	}

	if p.def != nil {
		if global.Env().IsDebug {
			log.Trace("case -> default")
		}
		ctx.AddFlowProcess(p.Name() + ":default")
		return p.def.Process(ctx)
	}
	ctx.AddFlowProcess(p.Name() + ":no-match")
	return nil
}

// matchValues mirrors the `in` condition's membership semantics (typed
// comparison via util.ContainsAnyInAnyIntArray), so `field`+`values` arms
// and `when: {in: ...}` behave identically.
func (p *CaseProcessor) matchValues(values conditions.ValuesMap, want []interface{}) bool {
	v, err := values.GetValue(p.field)
	if err != nil {
		return false
	}
	return util.ContainsAnyInAnyIntArray(v, want)
}

func (p *CaseProcessor) Name() string {
	return "case"
}

func (p *CaseProcessor) String() string {
	var sb strings.Builder
	sb.WriteString("case ")
	for i, arm := range p.branches {
		if i > 0 {
			sb.WriteString("; ")
		}
		if arm.hasValues {
			sb.WriteString(fmt.Sprintf("%v in %v", p.field, arm.values))
		}
		if arm.cond != nil {
			if arm.hasValues {
				sb.WriteString(" and ")
			}
			sb.WriteString(arm.cond.String())
		}
		sb.WriteString(" then ")
		sb.WriteString(arm.then.Name())
	}
	if p.def != nil {
		sb.WriteString("; default ")
		sb.WriteString(p.def.Name())
	}
	return sb.String()
}
