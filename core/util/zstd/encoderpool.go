// Copyright 2020 Mostyn Bramley-Moore.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package zstd

import (
	"io"
	"runtime"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// EncoderWrapper is a wrapper that embeds a *zstd.Encoder, and is safe for
// use in a sync.Pool.
type EncoderWrapper struct {
	*zstd.Encoder
}

// NewEncoderPool returns a sync.Pool that provides EncoderWrapper
// objects which embed a *zstd.Encoder. You probably want to include
// zstd.WithEncoderConcurrency(1) in the list of options.
func NewEncoderPool(options ...zstd.EOption) *sync.Pool {
	p := &sync.Pool{}

	p.New = func() interface{} {
		e, _ := zstd.NewWriter(nil, options...)
		ew := &EncoderWrapper{Encoder: e}

		runtime.SetFinalizer(ew, func(ew *EncoderWrapper) {
			ew.Encoder.Close()
		})

		return ew
	}

	return p
}

// EncoderPoolWrapper is a convenience wrapper for sync.Pool which only
// accepts and returns *EncoderWrapper's.
type EncoderPoolWrapper struct {
	pool *sync.Pool
}

// Get returns an *EncoderWrapper that has been Reset to use w.
func (e *EncoderPoolWrapper) Get(w io.Writer) *EncoderWrapper {
	ew := e.pool.Get().(*EncoderWrapper)
	ew.Reset(w)
	return ew
}

// Put returns an *EncoderWrapper to the pool.
func (e *EncoderPoolWrapper) Put(w *EncoderWrapper) {
	w.Reset(nil)
	e.pool.Put(w)
}

// NewEncoderPoolWapper returns an *EncoderPoolWrapper that provides
// *EncoderWrapper objects that do not need to be type asserted.
// As with NewEncoderPool, you probably want to include
// zstd.WithEncoderConcurrency(1) in the list of options.
func NewEncoderPoolWrapper(options ...zstd.EOption) *EncoderPoolWrapper {
	return &EncoderPoolWrapper{pool: NewEncoderPool(options...)}
}
