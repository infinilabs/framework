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

package util

import (
	"regexp"
	"strings"
)

var (
	// Must escape these special characters in query_string
	specials = []string{
		`+`, `-`, `=`, `>`, `<`, `!`, `(`, `)`, `{`, `}`, `[`, `]`,
		`^`, `"`, `~`, `*`, `?`, `:`, `/`,
	}

	logicalOpRegex = regexp.MustCompile(`(\|\||&&)`)
)

// CleanUserQuery escapes or removes special characters for safe use in ES query_string
func CleanUserQuery(input string) string {
	// Escape backslash first
	input = strings.ReplaceAll(input, `\`, `\\`)

	// Escape whole-token logical operators (&&, ||)
	input = logicalOpRegex.ReplaceAllStringFunc(input, func(op string) string {
		return `\` + op
	})

	// Escape all other individual characters
	for _, ch := range specials {
		input = strings.ReplaceAll(input, ch, `\`+ch)
	}

	// Normalize whitespace
	input = strings.TrimSpace(input)
	input = regexp.MustCompile(`\s+`).ReplaceAllString(input, " ")

	return input
}
