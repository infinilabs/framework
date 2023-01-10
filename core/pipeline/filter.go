package pipeline

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/lib/fasthttp"
	"strings"
)

type Filter interface {
	Name() string
	Filter(ctx *fasthttp.RequestCtx)
}

type Filters struct {
	List []Filter `json:"list,omitempty"`
}

func NewFilterList() *Filters {
	return &Filters{}
}

func NewFilter(cfg []*config.Config) (*Filters, error) {
	procs := NewFilterList()

	for _, procConfig := range cfg {
		// Handle if/then/else processor which has multiple top-level keys.
		if procConfig.HasField("if") {
			p, err := NewIfElseThenFilter(procConfig)
			if err != nil {
				return nil, errors.Wrap(err, "failed to make if/then/else processor")
			}
			procs.AddFilter(p)
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

		log.Trace("action:", actionName, ",", actionCfg)

		gen, exists := registry.filterReg[actionName]
		if !exists {
			var validActions []string
			for k := range registry.filterReg {
				validActions = append(validActions, k)

			}
			return nil, errors.Errorf("the processor %s does not exist. valid processors: %v", actionName, strings.Join(validActions, ", "))
		}

		//actionCfg.PrintDebugf("Configure processor action '%v' with:", actionName)
		constructor := gen.FilterPlugin()
		plugin, err := constructor(actionCfg)
		if err != nil||plugin==nil {
			return nil, err
		}

		p, ok := plugin.(Filter)
		if ok {
			procs.AddFilter(p)
		} else {
			return nil, errors.Errorf("invalid processor: [%v]", plugin.Name())
		}
	}

	if len(procs.List) > 0 {
		log.Debugf("generated new filters: %v", procs)
	}
	return procs, nil
}

func (procs *Filters) AddFilter(p Filter) {
	procs.List = append(procs.List, p)
}

func (procs *Filters) AddFilters(p Filters) {
	// Subtlety: it is important here that we append the individual elements of
	// p, rather than p itself, even though
	// p implements the processors.Processor interface. This is
	// because the contents of what we return are later pulled out into a
	// processing.group rather than a processors.Filters, and the two have
	// different error semantics: processors.Filters aborts processing on
	// any error, whereas processing.group only aborts on fatal errors. The
	// latter is the most common behavior, and the one we are preserving here for
	// backwards compatibility.
	// We are unhappy about this and have plans to fix this inconsistency at a
	// higher level, but for now we need to respect the existing semantics.
	procs.List = append(procs.List, p.List...)
}

func (procs *Filters) All() []Filter {
	if procs == nil || len(procs.List) == 0 {
		return nil
	}

	ret := make([]Filter, len(procs.List))
	for i, p := range procs.List {
		ret[i] = p
	}
	return ret
}

func (procs *Filters) Close() error {
	return nil
	//var errs multierror.Errors
	//for _, p := range procs.List {
	//	err := Close(p)
	//	if err != nil {
	//		errs = append(errs, err)
	//	}
	//}
	//return errs.Err()
}

func (procs *Filters) Filter(ctx *fasthttp.RequestCtx) {
	if procs==nil{
		log.Errorf("invalid filter: %v",procs)
		return
	}
	for _, p := range procs.List {

		if !ctx.ShouldContinue() {
			if global.Env().IsDebug {
				log.Tracef("filter [%v] not continued, position: [%v]", p.Name(),ctx.GetRequestProcess())
			}
			ctx.AddFlowProcess("skipped")
			return
		}

		ctx.AddFlowProcess(p.Name())

		if global.Env().IsDebug{
			log.Trace("processing:",p.Name()," OF ",ctx.GetFlowProcess())
		}

		p.Filter(ctx)
		//event, err = p.Filter(filterCfg,ctx)
		//if err != nil {
		//	return event, errors.Wrapf(err, "failed applying processor %v", p)
		//}
		//if event == nil {
		//	// Drop.
		//	return nil, nil
		//}
	}
}

func (procs *Filters) Name() string {
	return "filters"
}

func (procs *Filters) String() string {
	var s []string
	for _, p := range procs.List {
		s = append(s, p.Name())
	}
	return strings.Join(s, ", ")
}
