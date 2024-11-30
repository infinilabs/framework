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
	"fmt"
	"testing"
)

func TestStrip(t *testing.T) {
	
	const src = "déjà vu" + // precomposed unicode
	"\n\000\037 \041\176\177\200\377\n" + // various boundary cases
	"as⃝df̅" // unicode combining characters


	fmt.Println("source text:")
	fmt.Println(src)
	fmt.Println("\nas bytes, stripped of control codes:")
	fmt.Println(StripCtlFromBytes(src))
	fmt.Println("\nas bytes, stripped of control codes and extended characters:")
	fmt.Println(StripCtlAndExtFromBytes(src))
	fmt.Println("\nas UTF-8, stripped of control codes:")
	fmt.Println(StripCtlFromUTF8(src))
	fmt.Println("\nas UTF-8, stripped of control codes and extended characters:")
	fmt.Println(StripCtlAndExtFromUTF8(src))
	fmt.Println("\nas decomposed and stripped Unicode:")
	fmt.Println(StripCtlAndExtFromUnicode(src))
}
