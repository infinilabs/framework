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

package flag

import (
	"encoding/json"

	"infini.sh/framework/lib/go-ucfg"
	"infini.sh/framework/lib/go-ucfg/cfgutil"
)

type FlagValue struct {
	collector *cfgutil.Collector
	loader    func(arg string) (*ucfg.Config, error, error)
}

func newFlagValue(
	cfg *ucfg.Config,
	opts []ucfg.Option,
	loader func(string) (*ucfg.Config, error, error),
) *FlagValue {
	return &FlagValue{
		collector: cfgutil.NewCollector(cfg, opts...),
		loader:    loader,
	}
}

func (v *FlagValue) Config() *ucfg.Config {
	return v.collector.Config()
}

func (v *FlagValue) Error() error {
	return v.collector.Error()
}

func (v *FlagValue) String() string {
	if v.collector == nil {
		return ""
	}

	return toString(v.Config(), v.collector.GetOptions(), v.onError)
}

func (v *FlagValue) Get() interface{} {
	return v.Config()
}

func (v *FlagValue) Set(arg string) error {
	cfg, internalErr, reportErr := v.loader(arg)
	v.collector.Add(cfg, internalErr)
	return reportErr
}

func (v *FlagValue) onError(err error) error {
	return v.collector.Add(nil, err)
}

func toString(cfg *ucfg.Config, opts []ucfg.Option, onError func(error) error) string {
	var tmp map[string]interface{}
	if err := cfg.Unpack(&tmp, opts...); err != nil {
		return onError(err).Error()
	}

	js, err := json.Marshal(tmp)
	if err != nil {
		return onError(err).Error()
	}

	return string(js)
}
