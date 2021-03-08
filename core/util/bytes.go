/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unsafe"
)

// BytesToUint64 convert bytes to type uint64
func BytesToUint64(b []byte) (v uint64) {
	length := uint(len(b))
	for i := uint(0); i < length-1; i++ {
		v += uint64(b[i])
		v <<= 8
	}
	v += uint64(b[length-1])
	return
}

// BytesToUint32 convert bytes to uint32
func BytesToUint32(b []byte) (v uint32) {
	length := uint(len(b))
	for i := uint(0); i < length-1; i++ {
		v += uint32(b[i])
		v <<= 8
	}
	v += uint32(b[length-1])
	return
}

// Uint64toBytes convert uint64 to bytes
func Uint64toBytes(b []byte, v uint64) {
	for i := uint(0); i < 8; i++ {
		b[7-i] = byte(v >> (i * 8))
	}
}

// Uint32toBytes convert uint32 to bytes, max uint: 4294967295
func Uint32toBytes(b []byte, v uint32) {
	for i := uint(0); i < 4; i++ {
		b[3-i] = byte(v >> (i * 8))
	}
}

func Int64ToBytes(i int64) []byte {
	var buf = make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(i))
	return buf
}

func BytesToInt64(buf []byte) int64 {
	return int64(binary.BigEndian.Uint64(buf))
}

func IntToBytes(n int) []byte {
	data := int64(n)
	bytebuf := bytes.NewBuffer([]byte{})
	binary.Write(bytebuf, binary.BigEndian, data)
	return bytebuf.Bytes()
}

func BytesToInt(buf []byte) int {
	bytebuff := bytes.NewBuffer(buf)
	var data int64
	binary.Read(bytebuff, binary.BigEndian, &data)
	return int(data)
}

// DeepCopy return a deep copied object
func DeepCopy(value interface{}) interface{} {
	if valueMap, ok := value.(map[string]interface{}); ok {
		newMap := make(map[string]interface{})
		for k, v := range valueMap {
			newMap[k] = DeepCopy(v)
		}

		return newMap
	} else if valueSlice, ok := value.([]interface{}); ok {
		newSlice := make([]interface{}, len(valueSlice))
		for k, v := range valueSlice {
			newSlice[k] = DeepCopy(v)
		}

		return newSlice
	}

	return value
}

/** https://github.com/cloudfoundry/bytefmt/blob/master/bytes.go start
https://github.com/cloudfoundry/bytefmt/blob/master/LICENSE
Apache License  Version 2.0, January 2004
 **/

// ByteSize unit definition
const (
	BYTE     = 1.0
	KILOBYTE = 1024 * BYTE
	MEGABYTE = 1024 * KILOBYTE
	GIGABYTE = 1024 * MEGABYTE
	TERABYTE = 1024 * GIGABYTE
)

var bytesPattern *regexp.Regexp = regexp.MustCompile(`(?i)^(-?\d+)([KMGT]B?|B)$`)

var errInvalidByteQuantity = errors.New("Byte quantity must be a positive integer with a unit of measurement like M, MB, G, or GB")

// ByteSize returns a human-readable byte string of the form 10M, 12.5K, and so forth.  The following units are available:
//	T: Terabyte
//	G: Gigabyte
//	M: Megabyte
//	K: Kilobyte
//	B: Byte
// The unit that results in the smallest number greater than or equal to 1 is always chosen.
func ByteSize(bytes uint64) string {
	unit := ""
	value := float32(bytes)

	switch {
	case bytes >= TERABYTE:
		unit = "T"
		value = value / TERABYTE
	case bytes >= GIGABYTE:
		unit = "G"
		value = value / GIGABYTE
	case bytes >= MEGABYTE:
		unit = "M"
		value = value / MEGABYTE
	case bytes >= KILOBYTE:
		unit = "K"
		value = value / KILOBYTE
	case bytes >= BYTE:
		unit = "B"
	case bytes == 0:
		return "0"
	}

	stringValue := fmt.Sprintf("%.1f", value)
	stringValue = strings.TrimSuffix(stringValue, ".0")
	return fmt.Sprintf("%s%s", stringValue, unit)
}

// ToMegabytes parses a string formatted by ByteSize as megabytes.
func ToMegabytes(s string) (uint64, error) {
	bytes, err := ToBytes(s)
	if err != nil {
		return 0, err
	}

	return bytes / MEGABYTE, nil
}

// ToBytes parses a string formatted by ByteSize as bytes.
func ToBytes(s string) (uint64, error) {
	parts := bytesPattern.FindStringSubmatch(strings.TrimSpace(s))
	if len(parts) < 3 {
		return 0, errInvalidByteQuantity
	}

	value, err := strconv.ParseUint(parts[1], 10, 0)
	if err != nil || value < 1 {
		return 0, errInvalidByteQuantity
	}

	var bytes uint64
	unit := strings.ToUpper(parts[2])
	switch unit[:1] {
	case "T":
		bytes = value * TERABYTE
	case "G":
		bytes = value * GIGABYTE
	case "M":
		bytes = value * MEGABYTE
	case "K":
		bytes = value * KILOBYTE
	case "B":
		bytes = value * BYTE
	}

	return bytes, nil
}

/** https://github.com/cloudfoundry/bytefmt/blob/master/bytes.go end **/

func BytesToString(bs []byte) string {
	return *(*string)(unsafe.Pointer(&bs))
}

// ToLowercase convert string bytes to lowercase
func ToLowercase(str []byte) []byte {
	for i, s := range str {
		if s > 64 && s < 91 {
			str[i] = s + 32
		}
	}
	return str
}

// ToUppercase convert string bytes to uppercase
func ToUppercase(str []byte) []byte {
	for i, s := range str {
		if s > 96 && s < 123 {
			str[i] = s - 32
		}
	}
	return str
}

//TODO optimize performance
//ReplaceByte simply replace old bytes to new bytes, the two bytes should have same length
func ReplaceByte(str []byte, old, new []byte) []byte {
	return []byte(strings.Replace(string(str), string(old), string(new), -1))
}

//MustToJSONBytes convert interface to json with byte array
func MustToJSONBytes(v interface{}) []byte {
	b, err := ToJSONBytes(v)
	if err != nil {
		panic(err)
	}
	return b
}

func ToJSONBytes(v interface{}) ([]byte,error) {
	return json.Marshal(v)
}

//MustFromJSONBytes simply do json unmarshal
func MustFromJSONBytes(b []byte, v interface{}) {
	var err error
	err=FromJSONBytes(b,v)
	if err != nil {
		log.Error("data:", string(b))
		panic(err)
	}
}

func FromJSONBytes(b []byte, v interface{})(err error) {
	if b == nil || len(b) == 0 {
		return
	}
	err = json.Unmarshal(b, v)
	return err
}

func EncodeToBytes(key interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(key)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func GetBytes(key interface{}) []byte {
	return []byte(fmt.Sprintf("%v", key.(interface{})))
}

func GetSplitFunc(split []byte) func(data []byte, atEOF bool) (advance int, token []byte, err error) {

	sLen := len(split)

	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		dataLen := len(data)

		// Return nothing if at end of file and no data passed
		if atEOF && dataLen == 0 {
			return 0, nil, nil
		}

		// Find next separator and return token
		if i := bytes.Index(data, split); i >= 0 {
			return i + sLen, data[0:i], nil
		}

		// If we're at EOF, we have a final, non-terminated line. Return it.
		if atEOF {
			return dataLen, data, nil
		}

		// Request more data.
		return 0, nil, nil
	}
}

func ExtractFieldFromJson(data *[]byte, fieldStartWith []byte, fieldEndWith []byte, leftMustContain []byte) (bool, []byte) {
	return ExtractFieldFromJsonOrder(data, fieldStartWith, fieldEndWith, leftMustContain, false)
}

func ExtractFieldFromJsonOrder(data *[]byte, fieldStartWith []byte, fieldEndWith []byte, leftMustContain []byte, reverse bool) (bool, []byte) {
	scanner := bufio.NewScanner(bytes.NewReader(*data))
	scanner.Split(GetSplitFunc(fieldEndWith))
	var str []byte
	for scanner.Scan() {
		text := scanner.Bytes()
		if leftMustContain != nil && len(leftMustContain) > 0 {
			if bytes.Contains(text, leftMustContain) {
				str = text
				break
			}
		}
	}

	if len(str) > 0 {
		var offset int
		if reverse {
			offset = bytes.LastIndex(str, fieldStartWith)
		} else {
			offset = bytes.Index(str, fieldStartWith)
		}

		if offset > 0 && offset < len(str) {
			newStr := str[offset+len(fieldStartWith) : len(str)]
			return true, newStr
		}
	} else {
		log.Trace("input data doesn't contain the split bytes")
	}
	return false, nil
}

// []byte("gte"), []byte("strict_date_optional_time"), []byte("range")
//blockSplit 按照那个部分来进行划分多个区块；
//validBlockMustContain 区块必须包含的关键字
//reverse 是从区块的前面还是后面进行处理
//processBlockStartWithBlock 从什么地方开始处理
//processBlockEndWithBlock 从什么地方结束处理
//maxSpan 开始和结束位置的最大间隔，约束范围
//匹配位置的处理函数
func ProcessJsonData(data *[]byte, blockSplit []byte,limitBlockSize int, validBlockMustContain [][]byte, reverse bool,  matchBlockStartWith []byte, matchBlocksEndWith []byte,maxSpan int, matchedBlockProcessHandler func(matchedData []byte, globalStartOffset, globalEndOffset int)) bool {
	scanner := bufio.NewScanner(bytes.NewReader(*data))
	scanner.Split(GetSplitFunc(blockSplit))
	var str []byte
	block:=0
	index:=0
	hasValidBlock:=false
	for scanner.Scan() {
		text := scanner.Bytes()
		//fmt.Println("scan+")
		if len(text)>0{
			block++
		}
		index=index+len(text)
		if len(text)>limitBlockSize{
			text=text[0:limitBlockSize]
		}

		if len(validBlockMustContain)>0{
			invalid:=false
			for _,v:=range validBlockMustContain{
				if !bytes.Contains(text, v) {
					//fmt.Println("valid block+")
					invalid=true
				}
			}
			if !invalid{
				str = text
				hasValidBlock=true
				break
			}
		}else{
			hasValidBlock=true
		}

	}
	if!hasValidBlock{
		//fmt.Println("none valid block+")
		return false
	}

	//fmt.Println("index:",index-len(str),"block:",string(str))
	//fmt.Println("index:",index,"len(str):",len(str),",len(matchBlockStartWith):",len(matchBlockStartWith),",len(data):",len(*data),",block:",string(str))
	globalIndex:=len(*data)-len(str)//-len(matchBlockStartWith)
	//fmt.Println("global index:",globalIndex,",",string((*data)[globalIndex:]))
	if block==1{
		return false
	}

//TODO 处理一个 block 里面匹配多个 match 的情况，多个条件需要替换
	if len(str) > 0 {
		var startOffset int
		var endOffset int
		base:=len(matchBlockStartWith)
		//fmt.Println("reverse:",reverse)
		//fmt.Println("base:",base)
		if reverse {
			startOffset = bytes.LastIndex(str, matchBlockStartWith)
		} else {
			startOffset = bytes.Index(str, matchBlockStartWith)
		}

		startOffset=startOffset+base-len(matchBlockStartWith)

		if matchBlocksEndWith!=nil{
			endOffset = bytes.Index(str[startOffset:], matchBlocksEndWith)
			endOffset=startOffset+endOffset
		}

		if endOffset<=0{
			//fmt.Println("matchBlocksEndWith is nil:",matchBlocksEndWith)
			endOffset=len(str)
		}


		//fmt.Println("startOffset:",startOffset)
		//fmt.Println("endOffset:",endOffset)

		if endOffset<=startOffset{
			//fmt.Println("start offset < end offset")
			return false
		}

		//fmt.Println("span:",endOffset-startOffset)
		if maxSpan< (endOffset-startOffset){
			//fmt.Println("beyond max span:",endOffset-startOffset," vs ",maxSpan)
			return false
		}

		//fmt.Println("new block:",string(str[startOffset:endOffset]))
		//fmt.Println("new global block:",string((*data)[globalIndex+startOffset:globalIndex+endOffset]))

		if startOffset > 0 && startOffset < len(str) {
			matchedBlockProcessHandler(str[startOffset:endOffset],globalIndex+startOffset,globalIndex+endOffset)
			return true
		}
	} else {
		log.Trace("input data doesn't contain the split bytes")
	}
	return false
}

func IsBytesEndingWith(data *[]byte, ending []byte) bool {
	return IsBytesEndingWithOrder(data, ending, false)
}

func IsBytesEndingWithOrder(data *[]byte, ending []byte, reverse bool) bool {
	var offset int
	if reverse {
		offset = bytes.LastIndex(*data, ending)
	} else {
		offset = bytes.Index(*data, ending)
	}
	return len(*data)-offset <= len(ending)
}

func BytesSearchValue(data, startTerm, endTerm, searchTrim []byte) bool {
	index := bytes.Index(data, startTerm)
	leftData := data[index+len(startTerm):]
	endIndex := bytes.Index(leftData, endTerm)
	lastTerm := leftData[0:endIndex]

	if bytes.Contains(lastTerm, searchTrim) {
		return true
	}
	return false
}

var bufferPool *sync.Pool = &sync.Pool{
	New: func() interface{} {
		buff := &bytes.Buffer{}
		buff.Grow(512)
		return buff
	},
}

func BytesHasSuffix(left,right []byte) bool {

	if len(left)==0||len(right)==0{
		return false
	}

	//fmt.Println(len(left))
	//fmt.Println(len(right))
	//fmt.Println(string(left[len(left)-1]))

	if len(right)==1{
		if right[0]==left[len(left)-1]{
			return true
		}else{
			return false
		}
	}
	return bytes.HasSuffix(left, right)
}

func InsertBytesAfterField(data *[]byte, start,toBeSkipedBytes, end []byte, bytesToInsert []byte) []byte {

	matchStart := false
	matchEnd := false

	toBeMachedBuffer := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(toBeMachedBuffer)
	defer toBeMachedBuffer.Reset()

	for i, v := range *data {
		toBeMachedBuffer.WriteByte(v)
		//fmt.Println("gonna check:",string(toBeMachedBuffer.String()))
		if matchStart && matchEnd {
			break
		}

		if matchStart && !matchEnd {
			//skip unwanted bytes
			if toBeSkipedBytes!=nil&&len(toBeSkipedBytes)>0{
				if BytesHasSuffix(toBeMachedBuffer.Bytes(), toBeSkipedBytes) {
					toBeSkipedBytes=nil
					toBeMachedBuffer.Reset()
					continue
				}
			}

			//collecting data
			//check whether matched end
			if BytesHasSuffix(toBeMachedBuffer.Bytes(), end) {
				//fmt.Println("mateched end")
				matchEnd = true
				offset:=i+1
				start:=(*data)[:offset]
				left:=(*data)[offset:]
				//fmt.Println(string(""))
				//fmt.Println(string(start))
				//fmt.Println(string(bytesToInsert))
				//fmt.Println(string(left))
				if bytesToInsert != nil && len(bytesToInsert) > 0 {
					newbuf:=bufferPool.Get().(*bytes.Buffer)
					newbuf.Write(start)
					newbuf.Write(bytesToInsert)
					newbuf.Write(left)
					data:= newbuf.Bytes()
					newbuf.Reset()
					bufferPool.Put(newbuf)
					return data
				}
				toBeMachedBuffer.Reset()
			}
			continue
		}

		if !matchStart {
			if BytesHasSuffix(toBeMachedBuffer.Bytes(), start) {

				//fmt.Println("mateched start")

				matchStart = true
				toBeMachedBuffer.Reset()
				continue
			}
		}
	}
	return *data
}

func ExtractFieldFromBytes(data *[]byte, start, end []byte, removedFromValue []byte) []byte {
	return ExtractFieldFromBytesWitSkipBytes(data,start,nil,end,removedFromValue)
}
func ExtractFieldFromBytesWitSkipBytes(data *[]byte, start,toBeSkipedBytes, end []byte, removedFromValue []byte) []byte {

	matchStart := false
	matchEnd := false

	//buffer := bufferPool.Get().(*bytes.Buffer)
	toBeMachedBuffer := bufferPool.Get().(*bytes.Buffer)
	//defer bufferPool.Put(buffer)
	defer bufferPool.Put(toBeMachedBuffer)
	//defer buffer.Reset()
	defer toBeMachedBuffer.Reset()
	var value []byte

	for _, v := range *data {
		toBeMachedBuffer.WriteByte(v)

		//fmt.Println("going to check:",toBeMachedBuffer.String())

		if matchStart && matchEnd {
			//return buffer.Bytes()
			return value
		}

		if matchStart && !matchEnd {

			//skip unwanted bytes
			if toBeSkipedBytes!=nil&&len(toBeSkipedBytes)>0{
				if BytesHasSuffix(toBeMachedBuffer.Bytes(), toBeSkipedBytes) {
					toBeSkipedBytes=nil
					toBeMachedBuffer.Reset()
					continue
				}
			}

				//collecting data
			//check whether matched end
			if BytesHasSuffix(toBeMachedBuffer.Bytes(), end) {
				matchEnd = true
				toBeMachedBuffer.Reset()
			} else {
				filtered := false
				if removedFromValue != nil && len(removedFromValue) > 0 {
					for _, x := range removedFromValue {
						if v == x {
							filtered = true
							break
						}
					}
				}
				if !filtered {
					value = append(value, v)
					//buffer.WriteByte(v)
				}
			}
			continue
		}

		if !matchStart {
			if BytesHasSuffix(toBeMachedBuffer.Bytes(), start) {
				matchStart = true
				toBeMachedBuffer.Reset()
				continue
			}
		}
	}
	return nil
}

func CompareStringAndBytes(b []byte, s string) bool {
	if len(s) != len(b) {
		return false
	}
	for i, x := range b {
		if x != s[i] {
			return false
		}
	}
	return true
}

func LimitedBytesSearch(data []byte, term []byte,limit int) bool {
	buffer:=make([]byte,len(term))
	start:=false
	bufferOffset:=0
	for i,v:=range data{
		if i>limit{
			return false
		}
		if!start{
			if term[0]==v{
				start=true
				bufferOffset=0
				buffer=append(buffer,v)
			}
		}else{
			if  len(buffer)==len(term){
				return true
			}

			bufferOffset++
			if term[bufferOffset]==v{
				buffer=append(buffer,v)
			}else{
				start=false
				buffer=[]byte{}
			}
		}
	}
	return false
}
