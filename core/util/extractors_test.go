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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractString(t *testing.T) {
	input := "test"

	v, err := ExtractString(input)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, input, v)
}

func TestExtractBool(t *testing.T) {
	input := true

	v, err := ExtractBool(input)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, input, v)
}

func TestExtractInt(t *testing.T) {
	type test struct {
		i   interface{}
		o   int64
		err bool
	}
	tests := []test{
		{float32(1.0), 1, false},
		{float64(1.0), 1, false},
		{float32(1.2), 0, true},
		{float64(1.2), 0, true},
		{float64(-1.2), 0, true},
		{float64(-1), -1, false},
		{int(-1), -1, false},
	}

	for _, test := range tests {
		v, err := ExtractInt(test.i)
		if v != test.o || (test.err && err == nil) {
			t.Fatal(test.i, test.o, v, test.err)
		}
	}
}
