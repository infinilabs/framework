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
	"github.com/hashicorp/go-version"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/lib/bytebufferpool"
	"io"
	"math/rand"
	"net/url"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	. "strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf16"
)

// Index returns the index of the first occurrence of substr in str, or -1 if substr is not present.
func ContainStr(s, substr string) bool {
	return Index(s, substr) != -1
}

// HasPrefix checks if the string s starts with the substring substr.
func StringDefault(val, defaultV string) string {
	if val != "" {
		return val
	}
	return defaultV
}

// CompareInteger compares two interface{} values as integers and returns true if they are equal.
func CompareInteger(x interface{}, y interface{}) bool {
	if x == nil || y == nil {
		return false
	}

	if x == y {
		return true
	}

	var xint = InterfaceToInt(x)
	var yint = InterfaceToInt(y)
	if xint == yint {
		return true
	}
	return false
}

// InterfaceToInt converts an interface{} to an int, handling various numeric types.
func InterfaceToInt(y interface{}) int {
	var yint = 0
	ytyp := reflect.TypeOf(y)
	if ytyp == nil {
		return yint
	}
	switch ytyp.Kind() {
	case reflect.Int:
		yint = y.(int)
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
	case reflect.Float32:
		yint = int(y.(float32))
	case reflect.Float64:
		yint = int(y.(float64))
	}
	return yint
}

// ContainsAnyInAnyIntArray checks if the interface i is present in any of the int64 or string values in v.
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

// ContainsInAnyIntArray checks if the int64 i is present in any of the int64 values in v.
func ContainsInAnyIntArray(i int64, v []int64) bool {
	for _, x := range v {
		if i == x {
			return true
		}
	}
	return false
}

// ContainsInAnyInt32Array checks if the int i is present in any of the int32 values in v.
func ContainsInAnyInt32Array(i int, v []int) bool {
	for _, x := range v {
		if i == x {
			return true
		}
	}
	return false
}

// Contains checks if the string s contains the substring substr.
func AnyInArrayEquals(v []string, s string) bool {
	for _, k := range v {
		if s == k {
			return true
		}
	}
	return false
}

// ContainsAnyInArray checks if the string s contains any of the substrings in v.
func ContainsAnyInArray(s string, v []string) bool {
	for _, k := range v {
		if ContainStr(s, k) {
			return true
		}
	}
	return false
}

// SuffixAnyInArray checks if the string s ends with any of the suffixes in v.
func SuffixAnyInArray(s string, v []string) bool {
	for _, k := range v {
		if SuffixStr(s, k) {
			return true
		}
	}
	return false
}

// PrefixAnyInArray checks if the string s starts with any of the prefixes in v.
func PrefixAnyInArray(s string, v []string) bool {
	for _, k := range v {
		if PrefixStr(s, k) {
			return true
		}
	}
	return false
}

// HasPrefix checks if the string s starts with the substring substr.
func PrefixStr(s, substr string) bool {
	return HasPrefix(s, substr)
}

// HasPrefix checks if the string s starts with the substring substr.
func SuffixStr(s, substr string) bool {
	return HasSuffix(s, substr)
}

// Index returns the index of the first occurrence of substr in str, or -1 if substr is not present.
func StringToUTF16(s string) []uint16 {
	return utf16.Encode([]rune(s + "\x00"))
}

// Index returns the index of the first occurrence of substr in str, or -1 if substr is not present.
func SubStringWithSuffix(str string, length int, suffix string) string {
	if len(str) > length {
		str = SubString(str, 0, length) + suffix
	}
	return str
}

// Index returns the index of the first occurrence of substr in str, or -1 if substr is not present.
func UnicodeIndex(str, substr string) int {
	// Convert the string to []rune to handle double-byte characters correctly
	result := Index(str, substr)
	if result >= 0 {
		// If the substring is found, we need to ensure we get the correct character index
		prefix := []byte(str)[0:result]
		// Convert the prefix to []rune to handle double-byte characters correctly
		rs := []rune(string(prefix))
		// Get the length of the []rune slice to find the correct index
		result = len(rs)
	}

	return result
}

// SubString returns a substring of the input string starting from 'begin' index with specified 'length'.
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

// TrimSpace removes leading and trailing whitespace from a string.
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

// ToIndentJson converts an interface to a JSON string with indentation.
func ToIndentJson(in interface{}) string {
	return ToJson(in, true)
}

// ToJson converts an interface to a JSON string, optionally with indentation.
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

// FromJson converts a JSON string to a specified type.
func FromJson(str string, to interface{}) error {
	return json.Unmarshal([]byte(str), to)
}

// Int64ToString converts an int64 to a string.
func Int64ToString(num int64) string {
	return strconv.FormatInt(num, 10)
}

// IntToString converts an int to a string.
func IntToString(num int) string {
	return strconv.Itoa(num)
}

// ToInt64 converts a string to an int64, returning an error if conversion fails.
func ToInt64(str string) (int64, error) {
	return strconv.ParseInt(str, 10, 64)
}

// ToInt converts a string to an int, returning an error if conversion fails.
func ToInt(str string) (int, error) {
	if IndexAny(str, ".") > 0 {
		nonFractionalPart := Split(str, ".")
		return strconv.Atoi(nonFractionalPart[0])
	} else {
		return strconv.Atoi(str)
	}

}

// ToFloat64 converts a string to a float64, returning an error if conversion fails.
func GetRuntimeErrorMessage(r runtime.Error) string {
	if r != nil {
		return r.Error()
	}
	panic(errors.New("nil runtime error"))
}

// Replace replaces old with new in input string
func XSSHandle(src string) string {
	src = Replace(src, ">", "&lt; ", -1)
	src = Replace(src, ">", "&gt; ", -1)
	return src
}

// UrlEncode encodes a string for use in a URL.
func UrlEncode(str string) string {
	return url.QueryEscape(str)
}

// UrlDecode decodes a URL-encoded string.
func UrlDecode(str string) string {
	out, err := url.QueryUnescape(str)
	if err != nil {
		panic(err)
	}
	return out
}

// FilterSpecialChar removes special characters from a string and replaces them with spaces.
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

// Sha1Hash computes the SHA-1 hash of a string and returns it as a base64 URL-encoded string.
func Sha1Hash(str string) string {
	h := sha1.New()
	io.WriteString(h, str)
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

// TrimSpaces will trim space and line break
func TrimSpaces(str string) string {
	return TrimSpace(str)
}

// Replace replaces old with new in input string
func RemoveSpaces(str string) string {
	str = Replace(str, " ", "", -1)
	return str
}

// TrimPrefix will trim left side of the string
func TrimLeftStr(str string, left string) string {
	return TrimPrefix(str, left)
}

// TrimLeftStr will trim left side of the string
func TrimRightStr(str string, right string) string {
	return TrimSuffix(str, right)
}

// MD5digest computes the MD5 hash of a string and returns it as a hexadecimal string.
func MD5digest(str string) string {
	sum := md5.Sum([]byte(str))
	return hex.EncodeToString(sum[:])
}

// MD5digestBytes computes the MD5 hash of a byte slice and returns it as a [16]byte array.
func MD5digestBytes(b []byte) [16]byte {
	return md5.Sum(b)
}

// MD5digestString computes the MD5 hash of a byte slice and returns it as a hexadecimal string.
func MD5digestString(b []byte) string {
	sum := md5.Sum(b)
	return hex.EncodeToString(sum[:])
}

// PrintStringByteLines prints a 2D byte slice as a string with each line prefixed by its index.
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

// JoinInterfaceArray joins a slice of interface{} into a string with a specified delimiter.
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

// Join joins a slice of strings into a single string with a specified delimiter.
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

// JoinMapString joins a map of string keys and string values into a string with a specified delimiter.
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

// JoinMap joins a map with a specified delimiter between key-value pairs.
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

// JoinMap joins a map of string keys and interface{} values into a string with a specified delimiter.
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

// escape "\r\n" to "\\n"
func EscapeNewLine(input []byte) []byte {
	input = ReplaceByte(input, strCRLF, escapedStrCRLF)
	input = ReplaceByte(input, strCRLF1, escapedStrCRLF)
	return input
}

// ReplaceByte replaces old with new in input byte slice
func ToString(obj interface{}) string {
	if obj == nil {
		return ""
	}
	str, ok := obj.(string)
	if ok {
		return str
	}
	return fmt.Sprintf("%v", obj)
}

// convert a->b to a,b
func ConvertStringToMap(str string, splitter string) (k, v string, err error) {
	if Contains(str, splitter) {
		o := Split(str, splitter)
		return TrimSpaces(o[0]), TrimSpaces(o[1]), nil
	}
	return "", "", errors.New("invalid format")
}

// ReplaceByte replaces old with new in input byte slice
func RegexPatternMatch(pattern, value string) bool {
	reg, err := regexp.Compile(pattern)
	if err != nil {
		panic(err)
	}
	return reg.MatchString(value)
}

// VersionCompare compares two version strings and returns:
// -1 if v1 < v2
// 0 if v1 == v2
// 1 if v1 > v2
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

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

const (
	minLength = 8

	lowercaseChars = "abcdefghijklmnopqrstuvwxyz"
	uppercaseChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitChars     = "0123456789"
	specialChars   = "!@#$%^&*()-_=+[]{}|;:',.<>?/"

	allChars    = lowercaseChars + uppercaseChars + digitChars + specialChars
	simpleChars = lowercaseChars + digitChars
)

// GenerateRandomString generates a random string of specified length using lowercase letters and digits.
func GenerateRandomString(cnum int) string {
	if cnum <= 0 {
		return ""
	}
	result := make([]byte, cnum)
	for i := range result {
		result[i] = simpleChars[seededRand.Intn(len(simpleChars))]
	}
	return string(result)
}

// GenerateSecureString generates a random string of specified length using lowercase letters, uppercase letters, digits, and special characters.
func GenerateSecureString(cnum int) string {
	if cnum < minLength {
		cnum = minLength
	}

	result := make([]byte, cnum)
	result[0] = lowercaseChars[seededRand.Intn(len(lowercaseChars))]
	result[1] = uppercaseChars[seededRand.Intn(len(uppercaseChars))]
	result[2] = digitChars[seededRand.Intn(len(digitChars))]
	result[3] = specialChars[seededRand.Intn(len(specialChars))]

	for i := 4; i < cnum; i++ {
		result[i] = allChars[seededRand.Intn(len(allChars))]
	}

	seededRand.Shuffle(len(result), func(i, j int) {
		result[i], result[j] = result[j], result[i]
	})

	return string(result)
}

// ValidateSecure checks if the input string meets the security requirements:
// - Minimum length of 8 characters
// - At least one uppercase letter, one lowercase letter, one digit, and one special character (if must is true)
// - If must is false, at least one lowercase letter and one digit are required
// If must is true, it enforces all security requirements.
// If must is false, it only requires at least one lowercase letter and one digit.
func ValidateSecure(input string, opts ...bool) bool {
	// Determine the security policy from the optional parameter. Defaults to true.
	must := len(opts) == 0 || opts[0]

	if len(input) < minLength {
		return false
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, char := range input {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case ContainsRune(specialChars, char):
			hasSpecial = true
		}
	}
	if must {
		return hasUpper && hasLower && hasDigit && hasSpecial
	}
	return hasLower && hasDigit
}

// StringInArray checks if a string is present in a slice of strings.
func StringInArray(s []string, element string) bool {
	for _, v := range s {
		if v == element {
			return true
		}
	}
	return false
}

// StringArrayIntersection returns the intersection of two string slices.
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
