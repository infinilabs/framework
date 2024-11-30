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

//The MIT License (MIT)
//
//Copyright (c) 2014 Joe Shaw
//
//Permission is hereby granted, free of charge, to any person obtaining a copy
//of this software and associated documentation files (the "Software"), to deal
//in the Software without restriction, including without limitation the rights
//to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
//copies of the Software, and to permit persons to whom the Software is
//furnished to do so, subject to the following conditions:
//
//The above copyright notice and this permission notice shall be included in
//all copies or substantial portions of the Software.
//
//THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
//IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
//AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
//LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
//OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
//THE SOFTWARE.

package errors

import (
	"fmt"
	"testing"
)

func TestZeroErrors(t *testing.T) {
	var e Errors
	err := e.Err()
	if err != nil {
		t.Error("An empty Errors Err() method should return nil")
	}
}

func TestNonZeroErrors(t *testing.T) {
	var e Errors
	e = append(e, fmt.Errorf("An error"))
	err := e.Err()
	if err == nil {
		t.Error("A nonempty Errors Err() method should not return nil")
	}

	merr, ok := err.(*MultiError)
	if !ok {
		t.Error("Errors Err() method should return a *MultiError")
	}

	if len(merr.Errors) != 1 {
		t.Error("The MultiError Errors field was not of length 1")
	}

	if merr.Errors[0] != e[0] {
		t.Error("The Error in merr.Errors was not the original error instance provided")
	}

	if merr.Error() != "1 error: An error" {
		t.Error("MultiError (single) string was not as expected")
	}

	e = append(e, fmt.Errorf("Another error"))
	merr = e.Err().(*MultiError)
	if merr.Error() != "2 errors: An error; Another error" {
		t.Error("MultiError (multiple) string was not as expected")
	}
}
