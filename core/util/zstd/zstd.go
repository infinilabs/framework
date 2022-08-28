/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

//https://github.com/dgraph-io/badger/pull/1706/files
package zstd

import (
	"errors"
	"github.com/klauspost/compress/zstd"
	"infini.sh/framework/core/util"
	"io"
	"os"
	"path/filepath"
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


func DecompressFile(file,to string) error {
	abs,err:=filepath.Abs(file)
	if util.FileExists(to){
		return errors.New("target file exits, skip "+to)
	}

	tmp:=to+".tmp"
	if util.FileExists(tmp){
		return errors.New("temp file for target file was exits, skip: "+tmp)
	}

	writer, err := os.Create(tmp)
	if err != nil {
		return err
	}
	reader, err := os.Open(abs)
	if err != nil {
		return err
	}

	defer func() {
		if writer!=nil{
			writer.Close()
		}
		if reader!=nil{
			reader.Close()
		}
	}()

	err=ZSTDReusedDecompress(writer,reader)
	if err != nil && err.Error() != "unexpected EOF"  {
		e:=os.Remove(tmp)
		if e!=nil{
			panic(e)
		}
	}else{
		e:=os.Rename(tmp,to)
		if e!=nil{
			panic(e)
		}
	}
	return err
}

func CompressFile(file,to string)error  {

	if util.FileExists(to){
		return errors.New("target file exits, skip "+to)
	}

	tmp:=to+".tmp"
	if util.FileExists(tmp){
		return errors.New("temp file for target file was exits, skip: "+tmp)
	}

	abs,err:=filepath.Abs(file)
	writer, err := os.Create(tmp)
	if err != nil {
		return err
	}
	reader, err := os.Open(abs)
	if err != nil {
		return err
	}

	defer func() {
		if writer!=nil{
			writer.Close()
		}
		if reader!=nil{
			reader.Close()
		}
	}()

	err= ZSTDReusedCompress(writer,reader)
	if err!=nil{
		e:=os.Remove(tmp)
		if e!=nil{
			panic(e)
		}
	}else{
		e:=os.Rename(tmp,to)
		if e!=nil{
			panic(e)
		}
	}
	return err
}
