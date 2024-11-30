// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

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

package match

import (
	"regexp"
	"regexp/syntax"
)

func compile(r *syntax.Regexp) (stringMatcher, error) {
	switch {
	case isSubstringLiteral(r):
		s := string(r.Rune)
		return &substringMatcher{s, []byte(s)}, nil

	case isExactLiteral(r):
		s := string(r.Sub[1].Rune)
		return &equalsMatcher{s, []byte(s)}, nil

	case isAltLiterals(r):
		var literals [][]byte
		for _, sub := range r.Sub {
			literals = append(literals, []byte(string(sub.Rune)))
		}
		return &altSubstringMatcher{literals}, nil

	case isOneOfLiterals(r):
		var literals [][]byte
		for _, sub := range r.Sub[1].Sub {
			literals = append(literals, []byte(string(sub.Rune)))
		}
		return &oneOfMatcher{literals}, nil

	case isPrefixLiteral(r):
		s := []byte(string(r.Sub[1].Rune))
		return &prefixMatcher{s}, nil

	case isPrefixAltLiterals(r):
		var literals [][]byte
		for _, sub := range r.Sub[1].Sub {
			literals = append(literals, []byte(string(sub.Rune)))
		}
		return &altPrefixMatcher{literals}, nil

	case isPrefixNumDate(r):
		return compilePrefixNumDate(r)

	case isEmptyText(r):
		var m *emptyStringMatcher
		return m, nil

	case isEmptyTextWithWhitespace(r):
		var m *emptyWhiteStringMatcher
		return m, nil

	case isAnyMatch(r):
		var m *matchAny
		return m, nil

	default:

		r, err := regexp.Compile(r.String())
		if err != nil {
			return nil, err
		}
		return r, nil
	}
}

func compilePrefixNumDate(r *syntax.Regexp) (stringMatcher, error) {
	m := &prefixNumDate{}

	i := 1
	if r.Sub[i].Op == syntax.OpLiteral {
		m.prefix = []byte(string(r.Sub[i].Rune))
		i++
	}

	digitLen := func(r *syntax.Regexp) int {
		if r.Op == syntax.OpConcat {
			return len(r.Sub)
		}
		return 1
	}

	var digits []int
	var seps [][]byte

	digits = append(digits, digitLen(r.Sub[i]))
	i++

	for i < len(r.Sub) {
		lit := []byte(string(r.Sub[i].Rune))
		i++

		// capture literal suffix
		if i == len(r.Sub) {
			m.suffix = lit
			break
		}

		seps = append(seps, lit)
		digits = append(digits, digitLen(r.Sub[i]))
		i++
	}

	minLen := len(m.prefix) + len(m.suffix)
	for _, d := range digits {
		minLen += d
	}
	for _, sep := range seps {
		minLen += len(sep)
	}

	m.digits = digits
	m.seps = seps
	m.minLen = minLen

	return m, nil
}
