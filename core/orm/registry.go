package orm

import (
	"sort"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
)

var registeredSchemas = []util.KeyValue{}

func MustRegisterSchemaWithIndexName(t interface{}, index string) {
	err := RegisterSchemaWithIndexName(t, index)
	if err != nil {
		panic(err)
	}
}

func RegisterSchemaWithIndexName(t interface{}, index string) error {
	registeredSchemas = append(registeredSchemas, util.KeyValue{Key: index, Payload: t})
	return nil
}

func InitSchema() error {
	for _, v := range registeredSchemas {
		err := getHandler().RegisterSchemaWithName(v.Payload, v.Key)
		if err != nil {
			log.Error("register schema with index name failed: ", v.Key, " error: ", err)
			return err
		}
	}
	return nil
}

var handler ORM

func getHandler() ORM {
	if handler == nil {
		panic(errors.New("ORM handler is not registered"))
	}
	return handler
}

var adapters map[string]ORM

func Register(name string, h ORM) {
	if adapters == nil {
		adapters = map[string]ORM{}
	}
	_, ok := adapters[name]
	if ok {
		panic(errors.Errorf("ORM handler with same name: %v already exists", name))
	}

	adapters[name] = h

	handler = h

	log.Debug("register ORM handler: ", name)

}

type Operation string

const (
	OpGet    Operation = "get"
	OpCreate Operation = "create"
	OpUpdate Operation = "update"
	OpDelete Operation = "delete"
	OpSave   Operation = "save"

	OpSearch        Operation = "search"
	OpDeleteByQuery Operation = "delete_by_query"
)

var (
	preHooks    = map[Operation][]prioritizedHook{}
	postHooks   = map[Operation][]prioritizedHook{}
	searchHooks = map[Operation][]prioritizedSearchHook{}
)

type HookFunc func(ctx *Context, op Operation, model interface{}) (*Context, interface{}, error)
type SearchHookFunc func(ctx *Context, op Operation, qb *QueryBuilder) error

type prioritizedHook struct {
	Priority int
	Fn       HookFunc
}

type prioritizedSearchHook struct {
	Priority int
	Fn       SearchHookFunc
}

func RegisterDataOperationPreHook(priority int, fn HookFunc, ops ...Operation) {
	for _, op := range ops {
		preHooks[op] = append(preHooks[op], prioritizedHook{Priority: priority, Fn: fn})
		sort.SliceStable(preHooks[op], func(i, j int) bool {
			return preHooks[op][i].Priority < preHooks[op][j].Priority
		})
	}
}
func RegisterDataOperationPostHook(priority int, fn HookFunc, ops ...Operation) {
	for _, op := range ops {
		postHooks[op] = append(postHooks[op], prioritizedHook{Priority: priority, Fn: fn})
		sort.SliceStable(postHooks[op], func(i, j int) bool {
			return postHooks[op][i].Priority < postHooks[op][j].Priority
		})
	}
}

func RegisterSearchOperationHook(priority int, fn SearchHookFunc, ops ...Operation) {
	for _, op := range ops {
		searchHooks[op] = append(searchHooks[op], prioritizedSearchHook{Priority: priority, Fn: fn})
		sort.SliceStable(searchHooks[op], func(i, j int) bool {
			return searchHooks[op][i].Priority < searchHooks[op][j].Priority
		})
	}
}

func runDataOperationPreHooks(op Operation, ctx *Context, model interface{}) (*Context, interface{}, error) {
	var err error
	for _, h := range preHooks[op] {
		if ctx, model, err = h.Fn(ctx, op, model); err != nil {
			return ctx, model, err
		}
	}

	return ctx, model, nil
}

func runDataOperationPostHooks(op Operation, ctx *Context, model interface{}) (*Context, interface{}, error) {
	var err error
	for _, h := range postHooks[op] {
		if ctx, model, err = h.Fn(ctx, op, model); err != nil {
			return ctx, model, err
		}
	}

	return ctx, model, nil
}

func runSearchOperationHooks(op Operation, ctx *Context, qb *QueryBuilder) error {
	for _, h := range searchHooks[op] {
		if err := h.Fn(ctx, op, qb); err != nil {
			return err
		}
	}

	return nil
}
