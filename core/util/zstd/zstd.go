/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

//https://github.com/dgraph-io/badger/pull/1706/files
package zstd

import (
	"github.com/klauspost/compress/zstd"
	"io"
	"sync"
)

var (
	decoder *zstd.Decoder
	encoder *zstd.Encoder
	encOnce, decOnce sync.Once
)

// ZSTDDecompress decompresses a block using ZSTD algorithm.
func ZSTDDecompress(dst, src []byte) ([]byte, error) {
	decOnce.Do(func() {
		var err error
		decoder, err = zstd.NewReader(nil)
		if err!=nil{
			panic(err)
		}
	})
	return decoder.DecodeAll(src, dst[:0])
}


func ZSTDCompress(dst, src []byte, compressionLevel int) ([]byte, error) {
	encOnce.Do(func() {
		var err error
		level := zstd.EncoderLevelFromZstd(compressionLevel)
		encoder, err = zstd.NewWriter(nil, zstd.WithEncoderLevel(level))
		if err!=nil{
			panic(err)
		}
	})
	return encoder.EncodeAll(src, dst[:0]), nil
}

// ZSTDCompressBound returns the worst case size needed for a destination buffer.
// Klauspost ZSTD library does not provide any API for Compression Bound. This
// calculation is based on the DataDog ZSTD library.
// See https://pkg.go.dev/github.com/DataDog/zstd#CompressBound
func ZSTDCompressBound(srcSize int) int {
	lowLimit := 128 << 10 // 128 kB
	var margin int
	if srcSize < lowLimit {
		margin = (lowLimit - srcSize) >> 11
	}
	return srcSize + (srcSize >> 8) + margin

}


// Create a sync.Pool which returns wrapped *zstd.Decoder's.
var decoderPool = NewDecoderPoolWrapper(zstd.WithDecoderConcurrency(1))
var encoderPool = NewEncoderPoolWrapper(zstd.WithEncoderConcurrency(1),zstd.WithEncoderLevel(zstd.SpeedBetterCompression))

// ZSTDDecompress decompresses a block using ZSTD algorithm.
func ZSTDReusedDecompress(uncompressedDataWriter io.Writer, compressedDataReader io.Reader) (error) {

	decoder := decoderPool.Get(compressedDataReader)
	defer decoderPool.Put(decoder)

	_, err := io.Copy(uncompressedDataWriter, decoder)
	return err
}


func ZSTDReusedCompress(compressedDataWriter io.Writer, uncompressedDataReader io.Reader) (err error) {

	encoder := encoderPool.Get(compressedDataWriter)
	defer encoderPool.Put(encoder)

	_, err = io.Copy(encoder, uncompressedDataReader)

	return err
}
