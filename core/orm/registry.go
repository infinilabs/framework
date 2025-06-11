package orm

import (
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
