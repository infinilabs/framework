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

/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package zstd

import (
	"errors"
	"github.com/klauspost/compress/zstd"
	"infini.sh/framework/core/util"
	"io"
	"os"
	"path/filepath"
	log "github.com/cihub/seelog"
	"sync"
	"time"
)

var (
	encOnce, decOnce sync.Once
)

// Create a writer that caches compressors.
// For this operation type we supply a nil Reader.
var encoder, _ = zstd.NewWriter(nil,zstd.WithEncoderConcurrency(1))

// Compress a buffer.
// If you have a destination buffer, the allocation in the call can also be eliminated.
func CompressBytes(src []byte) []byte {
	return encoder.EncodeAll(src, make([]byte, 0, len(src)))
}

// Create a reader that caches decompressors.
// For this operation type we supply a nil Reader.
var decoder, _ = zstd.NewReader(nil, zstd.WithDecoderConcurrency(1))

// Decompress a buffer. We don't supply a destination buffer,
// so it will be allocated by the decoder.
func DecompressBytes(src []byte) ([]byte, error) {
	return decoder.DecodeAll(src, nil)
}

// Compress input to output.
func Compress(in io.Reader, out io.Writer) error {
	enc, err := zstd.NewWriter(out)
	if err != nil {
		return err
	}
	_, err = io.Copy(enc, in)
	if err != nil {
		enc.Close()
		return err
	}
	return enc.Close()
}

func Decompress(in io.Reader, out io.Writer) error {
	d, err := zstd.NewReader(in)
	if err != nil {
		return err
	}
	defer d.Close()

	// Copy content...
	_, err = io.Copy(out, d)
	return err
}


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

func CompressFile(file,to string)error  {

	if util.FileExists(to){
		return errors.New("target file exits, skip "+to)
	}

	tmp:=to+".tmp"
	if util.FileExists(tmp){
		info,err:=os.Stat(tmp)
		if err==nil{
			if time.Since(info.ModTime()).Seconds()<10{
				return errors.New("temp file for target file was exits, skip: "+tmp)
			}
		}
		err=os.Remove(tmp)
		if err!=nil{
			panic(err)
		}
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
			err:=writer.Close()
			if err!=nil{
				panic(err)
			}
		}
		if reader!=nil{
			err:=reader.Close()
			if err!=nil{
				panic(err)
			}
		}
	}()


	err=Compress(reader,writer)
	//err= ZSTDReusedCompress(writer,reader)
	if err!=nil{
		e:=os.Remove(tmp)
		if e!=nil{
			panic(e)
		}
	}else{
		e:=writer.Sync()
		if e!=nil{
			panic(e)
		}
		e=os.Rename(tmp,to)
		if e!=nil{
			panic(e)
		}
	}
	return err
}

func DecompressFile(locker *sync.RWMutex,file,to string) error {
	locker.Lock()
	defer locker.Unlock()
	abs,err:=filepath.Abs(file)
	if util.FileExists(to){
		log.Debug("target file exists, skip "+to)
		return nil
	}

	tmp:=to+".std_tmp"
	if util.FileExists(tmp){
		e:=os.Remove(tmp)
		if e!=nil{
			panic(e)
		}
		log.Warn("temp file for decompress zstd file was exists, delete: ",tmp)
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
			e:=writer.Close()
			if e!=nil{
				panic(e)
			}
		}
		if reader!=nil{
			e:=reader.Close()
			if e!=nil{
				panic(e)
			}
		}
	}()

	err=Decompress(reader,writer)
	//err=ZSTDReusedDecompress(writer,reader)
	if err != nil && err.Error() != "unexpected EOF"  {
		e:=os.Remove(tmp)
		if e!=nil{
			panic(e)
		}
	}else{
		e:=writer.Sync()
		if e!=nil{
			panic(e)
		}
		e=os.Rename(tmp,to)
		if e!=nil{
			panic(e)
		}
	}
	return err
}

