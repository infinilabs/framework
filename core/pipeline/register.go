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
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	p "infini.sh/framework/core/plugin"
	"reflect"
)

var typeRegistry = make(map[string]interface{})

func GetAllRegisteredJoints() map[string]interface{} {
	return typeRegistry
}

type JointType string

const INPUT JointType = "INPUT"
const OUTPUT JointType = "OUTPUT"
const FILTER JointType = "FILTER"
const PROCESSOR JointType = "PROCESSOR"

func GetJointInstance(cfg *ProcessorConfig) Processor {

	return getJoint(cfg).(Processor)
}

func GetInputJointInstance(cfg *ProcessorConfig) Input {

	return getJoint(cfg).(Input)
}

func GetOutputJointInstance(cfg *ProcessorConfig) Output {

	return getJoint(cfg).(Output)
}

func GetFilterJointInstance(cfg *ProcessorConfig) Filter {

	return getJoint(cfg).(Filter)
}

func getJoint(cfg *ProcessorConfig) interface{} {
	log.Tracef("get joint instances, %v", cfg.Name)
	if typeRegistry[cfg.Name] != nil {
		t := reflect.ValueOf(typeRegistry[cfg.Name]).Type()
		v := reflect.New(t).Elem()

		f := v.FieldByName("Data")
		if f.IsValid() && f.CanSet() && f.Kind() == reflect.Map {
			f.Set(reflect.ValueOf(cfg.Parameters))
		}
		return v.Interface()
	}
	panic(errors.New(cfg.Name + " not found"))
}

func RegisterPipeJoint(joint interface{}) {
	k := joint.(ProcessorBase).Name()
	RegisterPipeJointWithName(k, joint)
}

func RegisterPipeJointWithName(jointName string, joint interface{}) {
	if typeRegistry[jointName] != nil {
		panic(errors.Errorf("joint with same name already registered, %s", jointName))
	}
	typeRegistry[jointName] = joint
}


//new

type processorPlugin struct {
	name   string
	constr Constructor
}

var pluginKey = "basic.processor"

func Plugin(name string, c Constructor) map[string][]interface{} {
	return p.MakePlugin(pluginKey, processorPlugin{name, c})
}

func init() {
	p.MustRegisterLoader(pluginKey, func(ifc interface{}) error {
		p, ok := ifc.(processorPlugin)
		if !ok {
			return errors.New("plugin does not match processor plugin type")
		}

		return registry.Register(p.name, p.constr)
	})
}

type Constructor func(config *config.Config) (Processor, error)

var registry = NewNamespace()

func RegisterPlugin(name string, constructor Constructor) {
	log.Debugf("Register plugin %s", name)
	err := registry.Register(name, constructor)
	if err != nil {
		panic(err)
	}
}

func GetPlugin(name string)  {

}