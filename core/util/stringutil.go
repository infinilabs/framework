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

package util

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/lib/bytebufferpool"
	"io"
	"math/rand"
	"net/url"
	"reflect"
	"regexp"
	"runtime"
	"github.com/hashicorp/go-version"
	"strconv"
	. "strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf16"
)

func ContainStr(s, substr string) bool {
	return Index(s, substr) != -1
}

func StringDefault(val, defaultV string) string {
	if val != "" {
		return val
	}
	return defaultV
}

func CompareInteger(x interface{}, y interface{}) bool {

	if x == nil || y == nil {
		return false
	}

	if x == y {
		return true
	}

	var xint int = InterfaceToInt(x)
	var yint int = InterfaceToInt(y)
	//fmt.Println(xint,",",yint)
	if xint == yint {
		return true
	}
	return false
}

func InterfaceToInt(y interface{}) int {
	ytyp := reflect.TypeOf(y)
	var yint int = 0
	switch ytyp.Kind() {
	case reflect.Int:
		yint = int(y.(int))
	case reflect.Int8:
		yint = int(y.(int8))
	case reflect.Int16:
		yint = int(y.(int16))
	case reflect.Int32:
		yint = int(y.(int32))
	case reflect.Int64:
		yint = int(y.(int64))
	case reflect.Uint:
		yint = int(y.(uint))
	case reflect.Uint8:
		yint = int(y.(uint8))
	case reflect.Uint16:
		yint = int(y.(uint16))
	case reflect.Uint32:
		yint = int(y.(uint32))
	case reflect.Uint64:
		yint = int(y.(uint64))
	}
	return yint
}

func ContainsAnyInAnyIntArray(i interface{}, v []interface{}) bool {

	str, ok := i.(string)
	if ok {
		for _, x := range v {
			y, ok := x.(string)
			if ok {
				if str == y {
					return true
				}
			}
		}
		return false
	}

	for _, x := range v {
		if CompareInteger(i, x) {
			return true
		}
	}
	return false
}

func ContainsInAnyIntArray(i int64, v []int64) bool {
	for _, x := range v {
		if i == x {
			return true
		}
	}
	return false
}

func ContainsInAnyInt32Array(i int, v []int) bool {
	for _, x := range v {
		if i == x {
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

func SuffixAnyInArray(s string, v []string) bool {
	for _, k := range v {
		if SuffixStr(s, k) {
			return true
		}
	}
	return false
}

func PrefixAnyInArray(s string, v []string) bool {
	for _, k := range v {
		if PrefixStr(s, k) {
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
	lth := len(str)

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

	//safety for double characters
	safety := false
	if safety {
		rs := []rune(str)

		return string(rs[begin:end])
	} else {
		return str[begin:end]
	}

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

var locker sync.Mutex

func ToJson(in interface{}, indent bool) string {
	if in == nil {
		return ""
	}

	locker.Lock()
	defer locker.Unlock()

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

func Int64ToString(num int64) string {
	return strconv.FormatInt(num, 10)
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

func PrintStringByteLines(array [][]byte) string {
	buffer := bytes.Buffer{}
	x := len(array) - 1
	for i, v := range array {
		buffer.WriteString(fmt.Sprintf("%v:", i))
		buffer.Write(v)
		if i < x {
			buffer.WriteString("\n")
		}
	}
	return buffer.String()
}

func JoinInterfaceArray(array []interface{}, delimiter string, valueFunc func(str string) string) string {
	strs := []string{}
	for _, v := range array {
		v1 := ToString(v)
		if valueFunc != nil {
			v1 = valueFunc(v1)
		}
		strs = append(strs, v1)
	}
	return JoinArray(strs, delimiter)
}

func JoinArray(array []string, delimiter string) string {
	if len(array) < 100 {
		return Join(array, delimiter)
	}

	buffer := bytebufferpool.Get("array_join")
	defer bytebufferpool.Put("array_join", buffer)
	x := len(array) - 1
	for i, v := range array {
		buffer.WriteString(v)
		if i < x {
			buffer.WriteString(delimiter)
		}
	}
	return buffer.String()
}

func JoinMapString(array map[string]string, delimiter string) string {
	buffer := bytes.NewBuffer([]byte{})
	x := len(array) - 1
	i := 0
	for k, v := range array {
		buffer.WriteString(k)
		buffer.WriteString(delimiter)
		buffer.WriteString(ToString(v))
		if i < x {
			buffer.WriteString(objDelimiter)
		}
		i++
	}
	return buffer.String()
}

func JoinMapInt(array map[string]int, delimiter string) string {
	buffer := bytes.NewBuffer([]byte{})
	x := len(array) - 1
	i := 0
	for k, v := range array {
		buffer.WriteString(k)
		buffer.WriteString(delimiter)
		buffer.WriteString(ToString(v))
		if i < x {
			buffer.WriteString(objDelimiter)
		}
		i++
	}
	return buffer.String()
}

func JoinMap(array map[string]interface{}, delimiter string) string {
	buffer := bytes.NewBuffer([]byte{})
	x := len(array) - 1
	i := 0
	for k, v := range array {
		buffer.WriteString(k)
		buffer.WriteString(delimiter)
		buffer.WriteString(ToString(v))
		if i < x {
			buffer.WriteString(objDelimiter)
		}
		i++
	}
	return buffer.String()
}

var objDelimiter = ";"
var strCRLF = []byte("\r\n")
var strCRLF1 = []byte("\n")
var escapedStrCRLF = []byte("\\n")

//escape "\r\n" to "\\n"
func EscapeNewLine(input []byte) []byte {
	input = ReplaceByte(input, strCRLF, escapedStrCRLF)
	input = ReplaceByte(input, strCRLF1, escapedStrCRLF)
	return input
}

func ToString(obj interface{}) string {
	if obj==nil{
		return ""
	}
	str, ok := obj.(string)
	if ok {
		return str
	}
	return fmt.Sprintf("%v", obj)
}

//convert a->b to a,b
func ConvertStringToMap(str string, splitter string) (k, v string, err error) {
	if Contains(str, splitter) {
		o := Split(str, splitter)
		return TrimSpaces(o[0]), TrimSpaces(o[1]), nil
	}
	return "", "", errors.New("invalid format")
}

func RegexPatternMatch(pattern, value string) bool {
	reg, err := regexp.Compile(pattern)
	if err != nil {
		panic(err)
	}
	return reg.MatchString(value)
}

func VersionCompare(v1, v2 string) (int, error) {
	version1, err := version.NewVersion(v1)
	if err != nil {
		return -2, err
	}
	version2, err := version.NewVersion(v2)
	if err != nil {
		return -2, err
	}
	return version1.Compare(version2), nil
}
func GenerateRandomString(cnum int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < cnum; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}

	return string(result)
}
func StringInArray(s []string, element string) bool {
	for _, v := range s {
		if v == element {
			return true
		}
	}
	return false
}

func StringArrayIntersection(arr1 []string, arr2 []string) []string {
	strM := make(map[string]struct{}, len(arr1))
	for _, key := range arr1 {
		strM[key] = struct{}{}
	}
	var resultArr []string
	for _, key := range arr2 {
		if _, ok := strM[key]; ok {
			resultArr = append(resultArr, key)
		}
	}
	return resultArr
}
