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
	"github.com/infinitbyte/framework/core/errors"
	"reflect"
)

var typeRegistry = make(map[string]interface{})

func GetAllRegisteredJoints() map[string]interface{} {
	return typeRegistry
}

func GetJointInstance(cfg *JointConfig) Joint {
	log.Tracef("get joint instances, %v", cfg.JointName)
	if typeRegistry[cfg.JointName] != nil {
		t := reflect.ValueOf(typeRegistry[cfg.JointName]).Type()
		v := reflect.New(t).Elem()

		f := v.FieldByName("Data")
		if f.IsValid() && f.CanSet() && f.Kind() == reflect.Map {
			f.Set(reflect.ValueOf(cfg.Parameters))
		}
		v1 := v.Interface().(Joint)
		return v1
	}
	panic(errors.New(cfg.JointName + " not found"))
}

func RegisterPipeJoint(joint Joint) {
	k := string(joint.Name())
	RegisterPipeJointWithName(k, joint)
}

func RegisterPipeJointWithName(jointName string, joint Joint) {
	if typeRegistry[jointName] != nil {
		panic(errors.Errorf("joint with same name already registered, %s", jointName))
	}
	typeRegistry[jointName] = joint
}
