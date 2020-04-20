/*
Copyright 2016 Medcl (m AT medcl.net)

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
	"fmt"
	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"github.com/stretchr/testify/assert"
	"os"
	"reflect"
	"testing"
)

func TestPipelineConfig(t *testing.T) {

	util.RestorePersistID("/tmp")

	global.RegisterEnv(env.EmptyEnv())
	global.Env().IsDebug = true

	config := PipelineConfig{}
	context := &Context{}
	context.Set("url", "gogol.com")
	context.Set("webpage", "hello world gogo ")

	fmt.Println(util.ToJson(context, true))

	RegisterPipeJoint(crawlerJoint{})
	RegisterPipeJoint(parserJoint{})
	RegisterPipeJoint(saveJoint{})
	RegisterPipeJoint(publishJoint{})

	config.StartProcessor = &ProcessorConfig{Enabled: true, Name: "crawler", Parameters: map[string]interface{}{"url": "http://baidu12.com"}}
	joints := []*ProcessorConfig{}
	joints = append(joints, &ProcessorConfig{Enabled: true, Name: "parser", Parameters: map[string]interface{}{}})
	joints = append(joints, &ProcessorConfig{Enabled: true, Name: "save", Parameters: map[string]interface{}{}})
	joints = append(joints, &ProcessorConfig{Enabled: true, Name: "publish", Parameters: map[string]interface{}{}})

	config.Processors = joints

	pipe := NewPipelineFromConfig("test", &config, context)
	context = pipe.Run()

	fmt.Println("pipeline context")
	fmt.Println(context)
	fmt.Println(context.GetStringOrDefault("received_url", ""))

	assert.Equal(t, "http://baidu12.com", context.Data["received_url"])
	assert.Equal(t, "true", context.Data["published"])
	assert.Equal(t, "true", context.Data["saved"])
	assert.Equal(t, true, context.Data["status"])
	assert.Equal(t, "http://gogo.com", context.Data["host"])

}

type do interface {
	Do() string
}

type base struct {
	Para map[string]interface{}
}

type foo struct {
	base
	Id   int
	Name string
}

func (joint foo) Do() string {
	fmt.Println("foo do,", joint.Id, ",", joint.Name, ",", joint.Para)
	return joint.Name
}

func (joint bar) Do() string {
	fmt.Println("foo do")
	return ""
}

type bar struct {
}

func TestPipelineConfigReflection(t3 *testing.T) {
	var regStruct map[string]interface{}
	regStruct = make(map[string]interface{})
	regStruct["Foo"] = foo{Id: 1, Name: "medcl"}
	regStruct["Bar"] = bar{}

	str := "Bar"
	if regStruct[str] != nil {
		t := reflect.ValueOf(regStruct[str]).Type()
		v := reflect.New(t).Elem()
		fmt.Println(v)
		//v.MethodByName("Do").Call(nil)
	}

	//get another instance again
	str = "Foo"
	if regStruct[str] != nil {
		t := reflect.ValueOf(regStruct[str]).Type()
		v := reflect.New(t).Elem()
		fmt.Println(v)
		v1 := v.Interface().(do)
		v1.Do()
		assert.Equal(t3, "", v1.Do())
	}

	str = "Foo"
	if regStruct[str] != nil {
		t := reflect.ValueOf(regStruct[str]).Type()
		v := reflect.New(t).Elem()
		fmt.Println(v)

		f := v.FieldByName("Name")
		if f.IsValid() && f.CanSet() && f.Kind() == reflect.String {
			f.SetString("tom")
		}

		f = v.FieldByName("Id")
		if f.IsValid() && f.CanSet() && f.Kind() == reflect.Int {
			f.SetInt(55)
		}
		f = v.FieldByName("Para")
		fmt.Println(f.Kind())
		if f.IsValid() && f.CanSet() && f.Kind() == reflect.Map {
			para := map[string]interface{}{}
			para["key"] = "value123"
			f.Set(reflect.ValueOf(para))
		}

		fmt.Println(v)
		v1 := v.Interface().(do)
		v1.Do()
		assert.Equal(t3, "tom", v1.Do())

	}

	//get another instance again
	str = "Foo"
	if regStruct[str] != nil {
		t := reflect.ValueOf(regStruct[str]).Type()
		v := reflect.New(t).Elem()
		fmt.Println(v)
		v1 := v.Interface().(do)
		v1.Do()
		assert.Equal(t3, "", v1.Do())

	}

}

func TestNewPipelineFromConfig(t *testing.T) {
	path := "config_test.yml"
	config, err := yaml.NewConfigWithFile(path, ucfg.PathSep("."))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	pipeConfigs := struct {
		Pipelines []PipelineConfig `config:"pipelines"`
	}{}

	err = config.Unpack(&pipeConfigs)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	assert.Equal(t, 1, len(pipeConfigs.Pipelines))
	assert.Equal(t, "es_scroll", pipeConfigs.Pipelines[0].Name)
	assert.Equal(t, "es_scroll", pipeConfigs.Pipelines[0].StartProcessor.Name)
	assert.Equal(t, true, pipeConfigs.Pipelines[0].StartProcessor.Enabled)
	assert.Equal(t, "http://localhost:9200", pipeConfigs.Pipelines[0].StartProcessor.Parameters["endpoint"])
	assert.Equal(t, "elastic", pipeConfigs.Pipelines[0].StartProcessor.Parameters["username"])
	assert.Equal(t, "changeme", pipeConfigs.Pipelines[0].StartProcessor.Parameters["password"])
	assert.Equal(t, "twitter", pipeConfigs.Pipelines[0].StartProcessor.Parameters["index"])

	fmt.Println(pipeConfigs)

}

func TestGetStaticPipelineConfig(t *testing.T) {
	util.RestorePersistID("/tmp")

	global.RegisterEnv(env.EmptyEnv().SetConfigFile("config_test.yml"))

	p := GetStaticPipelineConfig("es_scroll")
	assert.Equal(t, "es_scroll", p.StartProcessor.Name)
	assert.Equal(t, true, p.StartProcessor.Enabled)
	assert.Equal(t, "http://localhost:9200", p.StartProcessor.Parameters["endpoint"])
	assert.Equal(t, "elastic", p.StartProcessor.Parameters["username"])
	assert.Equal(t, "changeme", p.StartProcessor.Parameters["password"])
	assert.Equal(t, "twitter", p.StartProcessor.Parameters["index"])

}
