/*
Copyright Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pipeline

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"strings"
)

type ProcessorBase interface {
	Name() string
}

type Processor interface {
	ProcessorBase
	Process(s *Context) error
}

type Processors struct {
	List []Processor
}

type ComplexProcessor interface {
	Processor
}

type Input interface {
	ProcessorBase
	Open() error
	Close() error
	Read() ([]byte, error)
}

type Output interface {
	ProcessorBase
	Open() error
	Close() error
	Write([]byte) error
}

type Filter interface {
	ProcessorBase
	Filter([]byte) error
}






type Closer interface {
	Close() error
}

func Close(p Processor) error {
	if closer, ok := p.(Closer); ok {
		return closer.Close()
	}
	return nil
}

func NewList() *Processors {
	return &Processors{}
}

func New(cfg PluginConfig) (*Processors, error) {
	procs := NewList()

	for _, procConfig := range cfg {
		// Handle if/then/else processor which has multiple top-level keys.
		if procConfig.HasField("if") {
			p, err := NewIfElseThenProcessor(procConfig)
			if err != nil {
				return nil, errors.Wrap(err, "failed to make if/then/else processor")
			}
			procs.AddProcessor(p)
			continue
		}

		if len(procConfig.GetFields()) != 1 {
			return nil, errors.Errorf("each processor must have exactly one "+
				"action, but found %d actions (%v)",
				len(procConfig.GetFields()),
				strings.Join(procConfig.GetFields(), ","))
		}

		actionName := procConfig.GetFields()[0]
		actionCfg, err := procConfig.Child(actionName, -1)
		if err != nil {
			return nil, err
		}

		//fmt.Println("get plugin:",actionName,actionCfg)

		gen, exists := registry.reg[actionName]
		if !exists {
			var validActions []string
			for k := range registry.reg {
				validActions = append(validActions, k)

			}
			return nil, errors.Errorf("the processor %s does not exist. valid processors: %v", actionName, strings.Join(validActions, ", "))
		}

		//actionCfg.PrintDebugf("Configure processor action '%v' with:", actionName)
		constructor := gen.Plugin()
		plugin, err := constructor(actionCfg)
		if err != nil {
			return nil, err
		}

		procs.AddProcessor(plugin)
	}

	if len(procs.List) > 0 {
		log.Debugf("Generated new processors: %v", procs)
	}
	return procs, nil
}

func (procs *Processors) AddProcessor(p Processor) {
	p1,ok:=p.(Processor)
	if !ok{
		panic("invalid processor")
	}
	procs.List = append(procs.List, p1)
}

func (procs *Processors) AddProcessors(p Processors) {
	// Subtlety: it is important here that we append the individual elements of
	// p, rather than p itself, even though
	// p implements the processors.Processor interface. This is
	// because the contents of what we return are later pulled out into a
	// processing.group rather than a processors.Processors, and the two have
	// different error semantics: processors.Processors aborts processing on
	// any error, whereas processing.group only aborts on fatal errors. The
	// latter is the most common behavior, and the one we are preserving here for
	// backwards compatibility.
	// We are unhappy about this and have plans to fix this inconsistency at a
	// higher level, but for now we need to respect the existing semantics.
	procs.List = append(procs.List, p.List...)
}

func (procs *Processors) All() []Processor {
	if procs == nil || len(procs.List) == 0 {
		return nil
	}

	ret := make([]Processor, len(procs.List))
	for i, p := range procs.List {
		ret[i] = p
	}
	return ret
}

func (procs *Processors) Close() error {
	var errs errors.Errors
	for _, p := range procs.List {
		err := Close(p)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs.Err()
}

// Run executes the all processors serially and returns the event and possibly
// an error. If the event has been dropped (canceled) by a processor in the
// list then a nil event is returned.
func (procs *Processors) Process(ctx *Context) error{
	for _, p := range procs.List {
		if !ctx.ShouldContinue(){
			if global.Env().IsDebug{
				log.Debugf("filter [%v] not continued",p.Name())
			}
			ctx.AddFlowProcess("skipped")
			return nil
		}
		ctx.AddFlowProcess(p.Name())
		log.Debug("start processing:",p.Name())
		err:=p.Process(ctx)
		//event, err = p.Process(filterCfg,ctx)
		if err != nil {
			log.Error("error on processing:",p.Name())
			return err
		}
		//if event == nil {
		//	// Drop.
		//	return nil, nil
		//}
	}
	return nil
}

func (procs *Processors) Name() string {
	return "filters"
}

func (procs *Processors) String() string {
	var s []string
	for _, p := range procs.List {
		s = append(s, p.Name())
	}
	return strings.Join(s, ", ")
}
