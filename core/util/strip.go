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
	"strings"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

//kudos to:
//https://rosettacode.org/wiki/Strip_control_codes_and_extended_characters_from_a_string

// two byte-oriented functions identical except for operator comparing c to 127.
func StripCtlFromBytes(str string) string {
	b := make([]byte, len(str))
	var bl int
	for i := 0; i < len(str); i++ {
		c := str[i]
		if c >= 32 && c != 127 {
			b[bl] = c
			bl++
		}
	}
	return string(b[:bl])
}

func StripCtlAndExtFromBytes(str string) string {
	b := make([]byte, len(str))
	var bl int
	for i := 0; i < len(str); i++ {
		c := str[i]
		if c >= 32 && c < 127 {
			b[bl] = c
			bl++
		}
	}
	return string(b[:bl])
}

// two UTF-8 functions identical except for operator comparing c to 127
func StripCtlFromUTF8(str string) string {
	return strings.Map(func(r rune) rune {
		if r == 9 || r == 10 || r == 13 {
			return r
		}

		if r >= 32 && r != 127 {
			return r
		}
		return -1
	}, str)
}

func StripCtlAndExtFromUTF8(str string) string {
	return strings.Map(func(r rune) rune {
		if r == 9 || r == 10 || r == 13 {
			return r
		}

		if r >= 32 && r < 127 {
			return r
		}
		return -1
	}, str)
}

// Advanced Unicode normalization and filtering,
// see http://blog.golang.org/normalization and
// http://godoc.org/golang.org/x/text/unicode/norm for more
// details.
func StripCtlAndExtFromUnicode(str string) string {
	isOk := func(r rune) bool {
		if r == 9 || r == 10 || r == 13 {
			return true
		}

		return r < 32 || r >= 127
	}
	// The isOk filter is such that there is no need to chain to norm.NFC
	t := transform.Chain(norm.NFKD, transform.RemoveFunc(isOk))
	// This Transformer could also trivially be applied as an io.Reader
	// or io.Writer filter to automatically do such filtering when reading
	// or writing data anywhere.
	str, _, _ = transform.String(t, str)
	return str
}
