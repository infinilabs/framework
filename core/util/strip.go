package util

import (
	"strings"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

//kudos to:
//https://rosettacode.org/wiki/Strip_control_codes_and_extended_characters_from_a_string

// two byte-oriented functions identical except for operator comparing c to 127.
func StripCtlFromBytes(str string) string {
	b := make([]byte, len(str))
	var bl int
	for i := 0; i < len(str); i++ {
		c := str[i]
		if c >= 32 && c != 127 {
			b[bl] = c
			bl++
		}
	}
	return string(b[:bl])
}
 
func StripCtlAndExtFromBytes(str string) string {
	b := make([]byte, len(str))
	var bl int
	for i := 0; i < len(str); i++ {
		c := str[i]
		if c >= 32 && c < 127 {
			b[bl] = c
			bl++
		}
	}
	return string(b[:bl])
}
 
// two UTF-8 functions identical except for operator comparing c to 127
func StripCtlFromUTF8(str string) string {
	return strings.Map(func(r rune) rune {
		if r==9||r==10||r==13{
			return r
		}

		if r >= 32 && r != 127 {
			return r
		}
		return -1
	}, str)
}
 
func StripCtlAndExtFromUTF8(str string) string {
	return strings.Map(func(r rune) rune {
		if r==9||r==10||r==13{
			return r
		}

		if r >= 32 && r < 127 {
			return r
		}
		return -1
	}, str)
}
 
// Advanced Unicode normalization and filtering,
// see http://blog.golang.org/normalization and
// http://godoc.org/golang.org/x/text/unicode/norm for more
// details.
func StripCtlAndExtFromUnicode(str string) string {
	isOk := func(r rune) bool {
		if r==9||r==10||r==13{
			return true
		}

		return r < 32 || r >= 127
	}
	// The isOk filter is such that there is no need to chain to norm.NFC
	t := transform.Chain(norm.NFKD, transform.RemoveFunc(isOk))
	// This Transformer could also trivially be applied as an io.Reader
	// or io.Writer filter to automatically do such filtering when reading
	// or writing data anywhere.
	str, _, _ = transform.String(t, str)
	return str
}
 