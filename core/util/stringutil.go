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
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"runtime"
	"strconv"
	. "strings"
	"unicode"
	"unicode/utf16"
)

func ContainStr(s, substr string) bool {
	return Index(s, substr) != -1
}

func CompareInterface(x interface{}, y interface{}) bool {

		if x == nil || y == nil {
			return false
		}

		var xint int = 0
		var yint int = 0

		xtyp := reflect.TypeOf(x)
		switch xtyp.Kind() {
		case reflect.Int:
			xint = int(x.(int))
		case reflect.Int32:
			xint = int(x.(int32))
		case reflect.Int16:
			xint = int(x.(int16))
		case reflect.Int64:
			xint = int(x.(int64))
		}

		ytyp := reflect.TypeOf(y)
		switch ytyp.Kind() {
		case reflect.Int:
			yint = int(y.(int))
		case reflect.Int32:
			yint = int(y.(int32))
		case reflect.Int16:
			yint = int(y.(int16))
		case reflect.Int64:
			yint = int(y.(int64))
		}

		if xint <= yint {
			return false
		}

		return true
}

func ContainsAnyInAnyIntArray(i interface{}, v []interface{}) bool {
	for _,x:=range v{
		//fmt.Println("checking..",i,x,i==x,CompareInterface(i,x),reflect.TypeOf(x),reflect.TypeOf(x))
		if CompareInterface(i,x){
			return true
		}
	}
	return false
}

func ContainsInAnyIntArray(i int64, v []int64) bool {
	for _,x:=range v{
		if i==x{
			return true
		}
	}
	return false
}

func ContainsAnyInArray(s string, v []string) bool {
	for _, k := range v {
		if ContainStr(s, k) {
			return true
		}
	}
	return false
}

func PrefixStr(s, substr string) bool {
	return HasPrefix(s, substr)
}

func SuffixStr(s, substr string) bool {
	return HasSuffix(s, substr)
}

func StringToUTF16(s string) []uint16 {
	return utf16.Encode([]rune(s + "\x00"))
}

func SubStringWithSuffix(str string, length int, suffix string) string {
	if len(str) > length {
		str = SubString(str, 0, length) + suffix
	}
	return str
}

func UnicodeIndex(str, substr string) int {
	// 子串在字符串的字节位置
	result := Index(str, substr)
	if result >= 0 {
		// 获得子串之前的字符串并转换成[]byte
		prefix := []byte(str)[0:result]
		// 将子串之前的字符串转换成[]rune
		rs := []rune(string(prefix))
		// 获得子串之前的字符串的长度，便是子串在字符串的字符位置
		result = len(rs)
	}

	return result
}

func SubString(str string, begin, length int) (substr string) {
	rs := []rune(str)
	lth := len(rs)

	if begin < 0 {
		begin = 0
	}
	if begin >= lth {
		begin = lth
	}
	end := begin + length
	if end > lth {
		end = lth
	}

	return string(rs[begin:end])
}

func NoWordBreak(in string) string {
	return Replace(in, "\n", " ", -1)
}

// Removes all unnecessary whitespaces
func MergeSpace(in string) (out string) {
	var buffer bytes.Buffer
	white := false
	for _, c := range in {
		if unicode.IsSpace(c) {
			if !white {
				buffer.WriteString(" ")
			}
			white = true
		} else {
			buffer.WriteRune(c)
			white = false
		}
	}
	return TrimSpace(buffer.String())
}

func ToJson(in interface{}, indent bool) string {
	if in == nil {
		return ""
	}
	var b []byte
	if indent {
		b, _ = json.MarshalIndent(in, " ", " ")
	} else {
		b, _ = json.Marshal(in)
	}
	return string(b)
}

func FromJson(str string, to interface{}) error {
	return json.Unmarshal([]byte(str), to)
}

func IntToString(num int) string {
	return strconv.Itoa(num)
}

func ToInt64(str string) (int64, error) {
	return strconv.ParseInt(str, 10, 64)
}

func ToInt(str string) (int, error) {
	if IndexAny(str, ".") > 0 {
		nonFractionalPart := Split(str, ".")
		return strconv.Atoi(nonFractionalPart[0])
	} else {
		return strconv.Atoi(str)
	}

}

func GetRuntimeErrorMessage(r runtime.Error) string {
	if r != nil {
		return r.Error()
	}
	panic(errors.New("nil runtime error"))
}

func XSSHandle(src string) string {
	src = Replace(src, ">", "&lt; ", -1)
	src = Replace(src, ">", "&gt; ", -1)
	return src
}

func UrlEncode(str string) string {
	return url.QueryEscape(str)
}

func UrlDecode(str string) string {
	out, err := url.QueryUnescape(str)
	if err != nil {
		panic(err)
	}
	return out
}

func FilterSpecialChar(keyword string) string {

	keyword = Replace(keyword, "\"", " ", -1)
	keyword = Replace(keyword, "+", " ", -1)
	keyword = Replace(keyword, "-", " ", -1)
	keyword = Replace(keyword, "/", " ", -1)
	keyword = Replace(keyword, "\\", " ", -1)
	keyword = Replace(keyword, ":", " ", -1)
	keyword = Replace(keyword, "?", " ", -1)
	keyword = Replace(keyword, "'", " ", -1)
	keyword = Replace(keyword, "[", " ", -1)
	keyword = Replace(keyword, "]", " ", -1)
	keyword = Replace(keyword, "{", " ", -1)
	keyword = Replace(keyword, "}", " ", -1)
	keyword = Replace(keyword, ")", " ", -1)
	keyword = Replace(keyword, "(", " ", -1)
	keyword = Replace(keyword, "~", " ", -1)
	keyword = Replace(keyword, "!", " ", -1)
	keyword = Replace(keyword, "›", " ", -1)
	keyword = Replace(keyword, ">", " ", -1)
	keyword = Replace(keyword, "<", " ", -1)
	keyword = Replace(keyword, "%", " ", -1)
	//keyword = Replace(keyword, ".", " ", -1)
	keyword = Replace(keyword, ",", " ", -1)
	keyword = Replace(keyword, "|", " ", -1)

	keyword = Replace(keyword, " 	  	  ", " ", -1)
	keyword = Replace(keyword, " 	  	", " ", -1)
	keyword = Replace(keyword, " 	  ", " ", -1)
	keyword = Replace(keyword, " 	 ", " ", -1)
	keyword = Replace(keyword, " 	", " ", -1)

	keyword = TrimSpace(keyword)
	return keyword
}

func Sha1Hash(str string) string {
	h := sha1.New()
	io.WriteString(h, str)
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

//TrimSpaces will trim space and line break
func TrimSpaces(str string) string {
	return TrimSpace(str)
}

func RemoveSpaces(str string) string {
	str = Replace(str, " ", "", -1)
	return str
}

func TrimLeftStr(str string, left string) string {
	return TrimPrefix(str, left)
}

func TrimRightStr(str string, right string) string {
	return TrimSuffix(str, right)
}

func MD5digest(str string) string {
	sum := md5.Sum([]byte(str))
	return hex.EncodeToString(sum[:])
}

func MD5digestBytes(b []byte) [16]byte {
	return md5.Sum(b)
}

func MD5digestString(b []byte) string {
	sum := md5.Sum(b)
	return hex.EncodeToString(sum[:])
}

func JoinArray(array []string,delimiter string) string {
	buffer:=bytes.NewBuffer([]byte{})
	x:=len(array)-1
	for i,v:=range array{
		buffer.WriteString(v)
		if i < x{
			buffer.WriteString(delimiter)
		}
	}
	return buffer.String()
}
var strCRLF=[]byte("\r\n")
var strCRLF1=[]byte("\n")
var escapedStrCRLF=[]byte("\\n")
//escape "\r\n" to "\\n"
func EscapeNewLine(input []byte)[]byte  {
	 input=ReplaceByte(input,strCRLF,escapedStrCRLF)
	 input=ReplaceByte(input,strCRLF1,escapedStrCRLF)
	 return input
}

func ToString(obj interface{})string  {
	return fmt.Sprintf("%v", obj)
}
