package util

import (
	"fmt"
	"testing"
)

func TestStrip(t *testing.T) {
	
	const src = "déjà vu" + // precomposed unicode
	"\n\000\037 \041\176\177\200\377\n" + // various boundary cases
	"as⃝df̅" // unicode combining characters


	fmt.Println("source text:")
	fmt.Println(src)
	fmt.Println("\nas bytes, stripped of control codes:")
	fmt.Println(StripCtlFromBytes(src))
	fmt.Println("\nas bytes, stripped of control codes and extended characters:")
	fmt.Println(StripCtlAndExtFromBytes(src))
	fmt.Println("\nas UTF-8, stripped of control codes:")
	fmt.Println(StripCtlFromUTF8(src))
	fmt.Println("\nas UTF-8, stripped of control codes and extended characters:")
	fmt.Println(StripCtlAndExtFromUTF8(src))
	fmt.Println("\nas decomposed and stripped Unicode:")
	fmt.Println(StripCtlAndExtFromUnicode(src))
}
