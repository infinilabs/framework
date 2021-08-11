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

// multierror is a simple Go package for combining multiple errors together.
package errors

import (
	"bytes"
	"fmt"
)

// The Errors type wraps a slice of errors
type Errors []error

// Returns a MultiError struct containing this Errors instance, or nil
// if there are zero errors contained.
func (e Errors) Err() error {
	if len(e) == 0 {
		return nil
	}

	return &MultiError{Errors: e}
}

// The MultiError type implements the error interface, and contains the
// Errors used to construct it.
type MultiError struct {
	Errors Errors
}

// Returns a concatenated string of the contained errors
func (m *MultiError) Error() string {
	var buf bytes.Buffer

	if len(m.Errors) == 1 {
		buf.WriteString("1 error: ")
	} else {
		fmt.Fprintf(&buf, "%d errors: ", len(m.Errors))
	}

	for i, err := range m.Errors {
		if i != 0 {
			buf.WriteString("; ")
		}

		buf.WriteString(err.Error())
	}

	return buf.String()
}
