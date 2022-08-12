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

package conditions

import (
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"infini.sh/framework/core/util/match"
	"io"
	"github.com/valyala/fasttemplate"
)

// Config represents a configuration for a condition, as you would find it in the config files.
type Config struct {
	Equals           *Fields                `config:"equals"`
	Contains         *Fields                `config:"contains"`
	Prefix           map[string]interface{} 	`config:"prefix"`
	Suffix           map[string]interface{}     `config:"suffix"`
	Regexp           *Fields                `config:"regexp"`
	Range            *Fields                `config:"range"`
	QueueHasLag      []string               `config:"queue_has_lag"`
	ConsumerHasLag   *Fields                `config:"consumer_has_lag"`
	ClusterAvailable []string               `config:"cluster_available"`
	HasFields        []string               `config:"has_fields"`
	Network          map[string]interface{} `config:"network"`
	OR               []Config               `config:"or"`
	AND              []Config               `config:"and"`
	NOT              *Config                `config:"not"`
	IN               map[string]interface{} `config:"in"`
}

// Condition is the interface for all defined conditions
type Condition interface {
	Check(event ValuesMap) bool
	String() string
}

// ValuesMap provides a common interface to read matchers for condition checking
type ValuesMap interface {
	// GetValue returns the given field from the map
	GetValue(string) (interface{}, error)
}

type MutableValueMap interface {
	PutValue(s string, value interface{}) (interface{}, error)
}

type RemovableValueMap interface {
	RemoveValue(s string)(bool, error)
}

type Context struct {
	contexts []ValuesMap
}

func (this *Context)AddContext(ctx ValuesMap)(*Context){
	if this.contexts==nil{
		this.contexts=[]ValuesMap{}
	}
	this.contexts=append(this.contexts,ctx)
	return this
}

func (this *Context)GetValue(k string) (interface{}, error) {

	//handle variables
	if util.ContainStr(k,"$[["){
		template, err := fasttemplate.NewTemplate(k, "$[[", "]]")
		if err != nil {
			panic(err)
		}
		k,err = template.ExecuteFuncStringWithErr(func(w io.Writer, tag string) (int, error) {
			variable,err := this.GetValue(tag)
			if err!=nil{
				return 0,err
			}
			return w.Write([]byte(util.ToString(variable)))
		})
		if err==nil{
			return this.GetValue(k)
		}
	}

	//check contexts
	for _,ctx:=range this.contexts{
		v,err:= ctx.GetValue(k)
		if err==nil{
			return v,err
		}
	}

	return nil, errors.Errorf("key=%v", k)
}


// NewCondition takes a Config and turns it into a real Condition
func NewCondition(config *Config) (Condition, error) {

	if config == nil {
		// empty condition
		return nil, errors.New("missing condition config")
	}

	var condition Condition
	var err error
	switch {
	case config.Equals != nil:
		condition, err = NewEqualsCondition(config.Equals.fields)
	case config.IN != nil:
		condition, err = NewInArrayCondition(config.IN)
	case config.Contains != nil:
		condition, err = NewMatcherCondition("contains", config.Contains.fields, match.CompileString)
	case config.Prefix != nil:
		condition, err = NewPrefixCondition(config.Prefix)
	case config.Suffix != nil:
		condition, err = NewSuffixCondition(config.Suffix)
	case config.Regexp != nil:
		condition, err = NewMatcherCondition("regexp", config.Regexp.fields, match.Compile)
	case config.Range != nil:
		condition, err = NewRangeCondition(config.Range.fields)
	case config.HasFields != nil:
		condition = NewHasFieldsCondition(config.HasFields)
	case config.QueueHasLag != nil:
		condition = NewQueueHasLagCondition(config.QueueHasLag)
	case config.ConsumerHasLag != nil:
		condition = NewConsumerHasLagCondition(config.ConsumerHasLag.fields)
	case config.ClusterAvailable != nil:
		condition = NewClusterAvailableCondition(config.ClusterAvailable)
	case config.Network != nil && len(config.Network) > 0:
		condition, err = NewNetworkCondition(config.Network)
	case len(config.OR) > 0:
		var conditionsList []Condition
		conditionsList, err = NewConditionList(config.OR)
		condition = NewOrCondition(conditionsList)
	case len(config.AND) > 0:
		var conditionsList []Condition
		conditionsList, err = NewConditionList(config.AND)
		condition = NewAndCondition(conditionsList)
	case config.NOT != nil:
		var inner Condition
		inner, err = NewCondition(config.NOT)
		if err == nil {
			condition, err = NewNotCondition(inner)
		}
	default:
		err = errors.New("missing or invalid condition")
	}
	if err != nil {
		return nil, err
	}

	return condition, nil
}

// NewConditionList takes a slice of Config objects and turns them into real Condition objects.
func NewConditionList(config []Config) ([]Condition, error) {
	out := make([]Condition, len(config))
	for i, condConfig := range config {
		cond, err := NewCondition(&condConfig)
		if err != nil {
			return nil, err
		}

		out[i] = cond
	}
	return out, nil
}
