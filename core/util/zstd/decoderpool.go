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

// Package syncpool provides a non-leaky sync.Pool for
// github.com/klauspost/compress/zstd's Encoder and Decoder types,
// using wrappers (EncoderWrapper and DecoderWrapper).
package zstd

import (
	"io"
	"runtime"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// DecoderWrapper is a wrapper that embeds a *zstd.Decoder, and is safe for
// use in a sync.Pool.
type DecoderWrapper struct {
	// *zstd.Decoder is not safe for use in a sync.Pool directly, since it
	// leaks data and goroutines. Finalizers on the *zstd.Decoder don't help
	// because the aforementioned goroutines reference the *zstd.Decoder and
	// prevent it from being garbage collected (so the finalizers don't run).
	//
	// We can work around this by storing this wrapper with an embedded
	// *zstd.Decoder in the sync.Pool, and using a finalizer on the wrapper
	// to Close the *zstd.Decoder.
	*zstd.Decoder

	// Instead of Closing a *zstd.Decoder, we can Reset it and return it
	// to this pool.
	pool *sync.Pool
}

// Close does not close the embedded *zstd.Decoder (once closed, they cannot
// be reused), but instead resets it and places this *DecoderWrapper back in
// the pool.
func (w *DecoderWrapper) Close() {
	err := w.Decoder.Reset(nil)
	if err == nil {
		w.pool.Put(w)
	}
}

// IOReadCloser returns an io.ReadCloser that will return this *DecoderWrapper
// to the pool when it is closed.
func (w *DecoderWrapper) IOReadCloser() io.ReadCloser {
	return &decoderReadCloser{
		dw:     w,
		Reader: w.Decoder.IOReadCloser(),
	}
}

type decoderReadCloser struct {
	dw *DecoderWrapper
	io.Reader
}

// Close does not close the underlying *zstd.Decoder, but instead returns
// it to the pool it came from.
func (w *decoderReadCloser) Close() error {
	w.dw.Close() // Returns the DecoderWrapper to the pool.
	return nil
}

// NewDecoderPool returns a sync.Pool that provides DecoderWrapper
// objects, which embed *zstd.Decoders. You probably want to include
// zstd.WithDecoderConcurrency(1) in the list of options.
func NewDecoderPool(options ...zstd.DOption) *sync.Pool {
	p := &sync.Pool{}

	p.New = func() interface{} {
		d, _ := zstd.NewReader(nil, options...)
		dw := &DecoderWrapper{
			Decoder: d,
			pool:    p,
		}

		runtime.SetFinalizer(dw, func(dw *DecoderWrapper) {
			dw.Decoder.Close()
		})

		return dw
	}

	return p
}

// DecoderPoolWrapper is a convenience wrapper for sync.Pool which only
// accepts and returns *DecoderWrapper's.
type DecoderPoolWrapper struct {
	pool *sync.Pool
}

// Get returns a *DecoderWrapper that has been Reset to use r.
func (d *DecoderPoolWrapper) Get(r io.Reader) *DecoderWrapper {
	dw := d.pool.Get().(*DecoderWrapper)

	err := dw.Reset(r)
	if err != nil {
		// Decoder.Reset only returns a non-nil error if Close has been
		// called. But DecoderWrapper.Close() intentionally doesn't call
		// Decoder.Close(), so this can only happen if a *DecoderWrapper
		// is type-asserted back to a *Decoder.
		panic(err)
	}

	return dw
}

// Put returns a *DecoderWrapper to the pool.
func (d *DecoderPoolWrapper) Put(w *DecoderWrapper) {
	err := w.Reset(nil)
	if err == nil {
		d.pool.Put(w)
	}
}

// NewDecoderPoolWapper returns a *DecoderPoolWrapper that provides
// *DecoderWrapper objects that do not need to be type asserted.
// As with NewDecoderPool, you probably want to include
// zstd.WithDecoderConcurrency(1) in the list of options.
func NewDecoderPoolWrapper(options ...zstd.DOption) *DecoderPoolWrapper {
	return &DecoderPoolWrapper{pool: NewDecoderPool(options...)}
}
