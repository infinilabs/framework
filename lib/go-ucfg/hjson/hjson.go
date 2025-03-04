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

package hjson

import (
	"io/ioutil"

	"gopkg.in/hjson/hjson-go.v3"

	"infini.sh/framework/lib/go-ucfg"
)

// NewConfig creates a new configuration object from the HJSON string passed via in.
func NewConfig(in []byte, opts ...ucfg.Option) (*ucfg.Config, error) {
	var m interface{}
	if err := hjson.Unmarshal(in, &m); err != nil {
		return nil, err
	}

	return ucfg.NewFrom(m, opts...)
}

// NewConfigWithFile loads a new configuration object from an external HJSON file.
func NewConfigWithFile(name string, opts ...ucfg.Option) (*ucfg.Config, error) {
	input, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}

	opts = append([]ucfg.Option{
		ucfg.MetaData(ucfg.Meta{Source: name}),
	}, opts...)
	return NewConfig(input, opts...)
}
