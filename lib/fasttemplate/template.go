// Package fasttemplate implements simple and fast template library.
//
// Fasttemplate is faster than text/template, strings.Replace
// and strings.Replacer.
//
// Fasttemplate ideally fits for fast and simple placeholders' substitutions.
package fasttemplate

import (
	"bytes"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/lib/bytebufferpool"
	"io"
	"strings"
)

// ExecuteFunc calls f on each template tag (placeholder) occurrence.
//
// Returns the number of bytes written to w.
//
// This function is optimized for constantly changing templates.
// Use Template.ExecuteFunc for frozen templates.
func ExecuteFunc(template, startTag, endTag string, w io.Writer, f TagFunc) (int64, error) {
	s := unsafeString2Bytes(template)
	a := unsafeString2Bytes(startTag)
	b := unsafeString2Bytes(endTag)

	var nn int64
	var ni int
	var err error
	for {
		n := bytes.Index(s, a)
		if n < 0 {
			break
		}
		ni, err = w.Write(s[:n])
		nn += int64(ni)
		if err != nil {
			return nn, err
		}

		s = s[n+len(a):]
		n = bytes.Index(s, b)
		if n < 0 {
			// cannot find end tag - just write it to the output.
			ni, _ = w.Write(a)
			nn += int64(ni)
			break
		}

		ni, err = f(w, unsafeBytes2String(s[:n]))
		nn += int64(ni)
		if err != nil {
			return nn, err
		}
		s = s[n+len(b):]
	}
	ni, err = w.Write(s)
	nn += int64(ni)

	return nn, err
}

// Execute substitutes template tags (placeholders) with the corresponding
// values from the map m and writes the result to the given writer w.
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// Returns the number of bytes written to w.
//
// This function is optimized for constantly changing templates.
// Use Template.Execute for frozen templates.
func Execute(template, startTag, endTag string, w io.Writer, m map[string]interface{}) (int64, error) {
	return ExecuteFunc(template, startTag, endTag, w, func(w io.Writer, tag string) (int, error) { return stdTagFunc(w, tag, m) })
}

// ExecuteStd works the same way as Execute, but keeps the unknown placeholders.
// This can be used as a drop-in replacement for strings.Replacer
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// Returns the number of bytes written to w.
//
// This function is optimized for constantly changing templates.
// Use Template.ExecuteStd for frozen templates.
func ExecuteStd(template, startTag, endTag string, w io.Writer, m map[string]interface{}) (int64, error) {
	return ExecuteFunc(template, startTag, endTag, w, func(w io.Writer, tag string) (int, error) { return keepUnknownTagFunc(w, startTag, endTag, tag, m) })
}

// ExecuteFuncString calls f on each template tag (placeholder) occurrence
// and substitutes it with the data written to TagFunc's w.
//
// Returns the resulting string.
//
// This function is optimized for constantly changing templates.
// Use Template.ExecuteFuncString for frozen templates.
func ExecuteFuncString(template, startTag, endTag string, f TagFunc) string {
	s, err := ExecuteFuncStringWithErr(template, startTag, endTag, f)
	if err != nil {
		panic(fmt.Sprintf("unexpected error: %s", err))
	}
	return s
}

// ExecuteFuncStringWithErr is nearly the same as ExecuteFuncString
// but when f returns an error, ExecuteFuncStringWithErr won't panic like ExecuteFuncString
// it just returns an empty string and the error f returned
func ExecuteFuncStringWithErr(template, startTag, endTag string, f TagFunc) (string, error) {
	tagsCount := bytes.Count(unsafeString2Bytes(template), unsafeString2Bytes(startTag))
	if tagsCount == 0 {
		return template, nil
	}

	//bb := bytebufferpool.Get("template")
	bb := &bytebufferpool.ByteBuffer{}
	if _, err := ExecuteFunc(template, startTag, endTag, bb, f); err != nil {
		bb.Reset()
		//bytebufferpool.Put("template", bb)
		return "", err
	}
	s := string(bb.B)
	//bb.Reset()
	//bytebufferpool.Put("template", bb)
	return s, nil
}

// ExecuteString substitutes template tags (placeholders) with the corresponding
// values from the map m and returns the result.
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// This function is optimized for constantly changing templates.
// Use Template.ExecuteString for frozen templates.
func ExecuteString(template, startTag, endTag string, m map[string]interface{}) string {
	return ExecuteFuncString(template, startTag, endTag, func(w io.Writer, tag string) (int, error) { return stdTagFunc(w, tag, m) })
}

// ExecuteStringStd works the same way as ExecuteString, but keeps the unknown placeholders.
// This can be used as a drop-in replacement for strings.Replacer
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// This function is optimized for constantly changing templates.
// Use Template.ExecuteStringStd for frozen templates.
func ExecuteStringStd(template, startTag, endTag string, m map[string]interface{}) string {
	return ExecuteFuncString(template, startTag, endTag, func(w io.Writer, tag string) (int, error) { return keepUnknownTagFunc(w, startTag, endTag, tag, m) })
}

// RenderNestedString creates a Template object and performs multi-pass rendering, discarding unknown placeholders.
// This is a top-level function for convenience.
// It combines template creation and multi-pass rendering.
// Returns the final rendered string or an error.
func RenderNestedString(template, startTag, endTag string, m map[string]interface{}) (string, error) {
	return executeNestedLoop(template, startTag, endTag, m, false)
}

// RenderNestedStringStd creates a Template object and performs multi-pass rendering, keeping unknown placeholders.
// This is a top-level function for convenience.
// It combines template creation and multi-pass rendering.
// Returns the final rendered string or an error.
func RenderNestedStringStd(template, startTag, endTag string, m map[string]interface{}) (string, error) {
	return executeNestedLoop(template, startTag, endTag, m, true)
}

// executeNestedLoop handles the core multi-pass loop logic as a top-level function.
// It creates/reuses a Template object internally for parsing and single-pass execution.
// Returns the final rendered string and an error if parsing or execution fails.
func executeNestedLoop(template, startTag, endTag string, m map[string]interface{}, keepUnknown bool) (string, error) {
	currentRendered := template // Start with the initial template string

	// Create a single Template object to reuse across iterations.
	var tempTpl Template // Create a zero-valued Template object

	for {
		// 1. Reset the *reused* Template object's parser state with the current template string.
		//    Need to pass template, startTag, endTag to Reset as it's now a method on tempTpl.
		//    Based on your Reset signature: func (t *Template) Reset(template, startTag, endTag string) error
		//    It seems Reset *expects* to take these as arguments. Let's revert Reset signature to take args.
		//    Or adjust Reset to use internal fields (t.template etc.). Let's adjust Reset to use internal fields as intended by Template method design.

		//    Let's assume Reset uses internal fields.
		//    Need to set the template, startTag, endTag on the reused object *before* calling Reset.
		tempTpl.template = currentRendered // Set the string to parse for this pass
		tempTpl.startTag = startTag        // Set start tag for parsing
		tempTpl.endTag = endTag            // Set end tag for parsing

		err := tempTpl.Reset(currentRendered, startTag, endTag) // Call Reset on the reused object
		if err != nil {
			// If parsing fails at any step, abort.
			log.Errorf("Nested rendering failed during parsing of intermediate template %q: %v", currentRendered, err) // Use currentRendered
			return currentRendered, fmt.Errorf("nested rendering parsing error: %w", err)
		}

		// 2. Execute one pass of substitution using the *reused* Template object.
		//    Use the methods that return error.
		//    ExecuteFuncStringWithErr operates on tempTpl's pre-parsed state.
		var nextRendered string
		var execErr error
		var tagFunc TagFunc // TagFunc to use for this pass

		// Select the appropriate TagFunc based on keepUnknown
		if keepUnknown {
			tagFunc = func(w io.Writer, tag string) (int, error) { return keepUnknownTagFunc(w, startTag, endTag, tag, m) } // Capture start/endTag, m
		} else {
			tagFunc = func(w io.Writer, tag string) (int, error) { return stdTagFunc(w, tag, m) } // Capture m
		}

		// Call ExecuteFuncStringWithErr on the reused Template object with the chosen TagFunc
		nextRendered, execErr = tempTpl.ExecuteFuncStringWithErr(tagFunc)

		if execErr != nil {
			// Pass up any errors from the single pass execution (e.g., a panic caught in ExecuteFunc and wrapped as error)
			log.Errorf("Nested rendering failed during execution pass for template %q: %v", currentRendered, execErr) // Use currentRendered
			return currentRendered, fmt.Errorf("nested rendering execution error: %w", execErr)                       // Return the state before failure and the error
		}

		// 3. Check for termination condition 1: No changes occurred (reached a fixed point)
		if nextRendered == currentRendered {
			return nextRendered, nil
		}

		// 4. Check for termination condition 2: No more potential tag structures left.
		//    Check on the *next* rendered string using the *original* startTag.
		hasStartTag := strings.Contains(nextRendered, startTag) // Use startTag

		if !hasStartTag {
			// No more start tags means rendering is complete.
			return nextRendered, nil
		}

		// 5. If changes occurred and potential tags still exist, continue the loop with the next rendered string.
		currentRendered = nextRendered

		// Optional: Add max passes limit here if needed
		// maxPasses := 10 // Example limit
		// if passCount > maxPasses {
		//     log.Warningf("Nested rendering reached max passes (%d) for template %q", maxPasses, template) // Use original template for log
		//     return currentRendered, fmt.Errorf("nested rendering exceeded maximum passes")
		// }
		// passCount++ // Increment pass count
	}
}

// Template implements simple template engine, which can be used for fast
// tags' (aka placeholders) substitution.
type Template struct {
	template string
	startTag string
	endTag   string

	texts [][]byte
	tags  []string
}

// New parses the given template using the given startTag and endTag
// as tag start and tag end.
//
// The returned template can be executed by concurrently running goroutines
// using Execute* methods.
//
// New panics if the given template cannot be parsed. Use NewTemplate instead
// if template may contain errors.
func New(template, startTag, endTag string) *Template {
	t, err := NewTemplate(template, startTag, endTag)
	if err != nil {
		panic(err)
	}
	return t
}

// NewTemplate parses the given template using the given startTag and endTag
// as tag start and tag end.
//
// The returned template can be executed by concurrently running goroutines
// using Execute* methods.
func NewTemplate(template, startTag, endTag string) (*Template, error) {
	var t Template
	err := t.Reset(template, startTag, endTag)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// TagFunc can be used as a substitution value in the map passed to Execute*.
// Execute* functions pass tag (placeholder) name in 'tag' argument.
//
// TagFunc must be safe to call from concurrently running goroutines.
//
// TagFunc must write contents to w and return the number of bytes written.
type TagFunc func(w io.Writer, tag string) (int, error)

func printByteArray(i [][]byte) {
	for x, v := range i {
		log.Info(x, ": ", string(v))
	}
}

func (t *Template) Reset(template, startTag, endTag string) error {
	// Keep these vars in t, so GC won't collect them and won't break
	// vars derived via unsafe*
	t.template = template
	t.startTag = startTag
	t.endTag = endTag
	t.texts = t.texts[:0]
	t.tags = t.tags[:0]

	if len(startTag) == 0 {
		panic("startTag cannot be empty")
	}
	if len(endTag) == 0 {
		panic("endTag cannot be empty")
	}

	s := unsafeString2Bytes(template)
	a := unsafeString2Bytes(startTag)
	b := unsafeString2Bytes(endTag)

	tagsCount := bytes.Count(s, a)
	if tagsCount == 0 {
		return nil
	}

	if tagsCount+1 > cap(t.texts) {
		t.texts = make([][]byte, 0, tagsCount+1)
	}
	if tagsCount > cap(t.tags) {
		t.tags = make([]string, 0, tagsCount)
	}

	for {
		n := bytes.Index(s, a)
		if n < 0 {
			t.texts = append(t.texts, s)
			break
		}

		//hit start tag, but maybe not correct one

		//let's find the first end tag as well
		endOffset := bytes.Index(s, b)
		//log.Error("first end offset: ",endOffset,",first start offset:",n)

		if endOffset < 0 {
			return fmt.Errorf("Cannot find end tag=%q in the template=%q starting from %q", endTag, template, s)
		}

		//end offset should be larger than start offset
		if endOffset < n {
			t.texts = append(t.texts, s)
			//log.Error("end offset is smaller than start offset, skipping")
			break
		}

		//fmt.Println("intial start offset:", n, ",end offset:", endOffset, ",len of s:", len(s), ",n+len(a)", n+len(a))

		//is there any first tags between start and end?
		//valid tag can be find
		if endOffset > n+len(a) && endOffset < len(s) {
			tagPart := s[n+len(a) : endOffset]
			//if there is, we need to find the last start tag offset
			if bytes.Contains(tagPart, a) {
				lastStartOffset := bytes.LastIndex(tagPart, a)
				moveRight := lastStartOffset + len(a)
				finalStartOffset := n + moveRight
				n = finalStartOffset
				//log.Error("last start offset: ",lastStartOffset,",",moveRight,",final:",finalStartOffset)
			}
		}

		t.texts = append(t.texts, s[:n])

		s = s[n+len(a):]
		n = bytes.Index(s, b)
		if n < 0 {
			return fmt.Errorf("Cannot find end tag=%q in the template=%q starting from %q", endTag, template, s)
		}

		t.tags = append(t.tags, unsafeBytes2String(s[:n]))
		s = s[n+len(b):]
	}

	return nil
}

// Reset resets the template t to new one defined by
// template, startTag and endTag.
//
// Reset allows Template object re-use.
//
// Reset may be called only if no other goroutines call t methods at the moment.
func (t *Template) Reset1(template, startTag, endTag string) error {
	// Keep these vars in t, so GC won't collect them and won't break
	// vars derived via unsafe*
	t.template = template
	t.startTag = startTag
	t.endTag = endTag
	t.texts = t.texts[:0]
	t.tags = t.tags[:0]

	if len(startTag) == 0 {
		panic("startTag cannot be empty")
	}
	if len(endTag) == 0 {
		panic("endTag cannot be empty")
	}

	s := unsafeString2Bytes(template)
	a := unsafeString2Bytes(startTag)
	b := unsafeString2Bytes(endTag)
	tagsCount := bytes.Count(s, a)
	//log.Error("input:",string(s),",start:",string(a),",end:",string(b),",tags count:",tagsCount)
	if tagsCount == 0 {
		return nil
	}

	if tagsCount+1 > cap(t.texts) {
		t.texts = make([][]byte, 0, tagsCount+1)
	}
	if tagsCount > cap(t.tags) {
		t.tags = make([]string, 0, tagsCount)
	}

	for {
		startOffset := bytes.Index(s, a)
		//log.Error("hit tag start:",startOffset,",",string(s),",",string(a))
		if startOffset < 0 {
			//log.Error("not containing any tag",string(s),",",string(a))
			t.texts = append(t.texts, s)
			break
		}

		s1 := s[startOffset+len(a):]
		//log.Error("new string:",string(s1))

		n := bytes.Index(s1, b) //the first ]] of matched end tag

		//log.Error("hit tag end, n:",n, ", s1:",string(s1),",b:",string(b),", s:",string(s))

		if n < 0 {
			return fmt.Errorf("Cannot find end tag=%q in the template=%q starting from %q", endTag, template, s)
		}

		tag := unsafeBytes2String(s1[:n]) //the tag name, but it may contain another tag
		//log.Error("hit tag:",tag)

		//if bytes.Index([]byte(tag), a) >= 0 {
		//	log.Error("hit tag start in tag:",tag,",",string(a))
		//}

		//check if contain start tag in tag
		o := bytes.LastIndex(s1[:n], a)
		//log.Error("hit tag start in tag:",string(s1[:n])," contains ",string(a),",",o,",",startOffset)
		if o >= 0 {
			prefix := s1[:o+len(a)]
			//log.Error("contain another tag in tag:",tag,",",string(s[:o+len(a)]),",prefix:",string(prefix))
			t.texts = append(t.texts, prefix)
			//log.Error("old t.texts:",o,",",startOffset,",",string(s1[:o+len(a)]))

			newTag := tag[o+len(a):]
			//log.Error(newTag,", texts:",string(prefix))
			t.tags = append(t.tags, newTag)
		} else {
			txt := s[:startOffset]
			t.texts = append(t.texts, txt)
			tag := unsafeBytes2String(s1[:n])
			//log.Error(tag,", texts:",string(txt))

			t.tags = append(t.tags, tag)
		}
		//fmt.Println("tags:",t.tags)

		s = s1[n+len(b):]
		//fmt.Println("final:",string(s))
	}

	//for i, tag := range t.texts {
	//	fmt.Println("texts:",i,",",string(tag))
	//}

	return nil
}

// ExecuteFunc calls f on each template tag (placeholder) occurrence.
//
// Returns the number of bytes written to w.
//
// This function is optimized for frozen templates.
// Use ExecuteFunc for constantly changing templates.
func (t *Template) ExecuteFunc(w io.Writer, f TagFunc) (int64, error) {
	var nn int64

	n := len(t.texts) - 1
	if n == -1 {
		ni, err := w.Write([]byte(t.template))
		return int64(ni), err
	}

	//log.Error(n," ",len(t.texts)," ",len(t.tags))
	//for i := 0; i < len(t.texts); i++ {
	//	log.Error(i,", text:",string(t.texts[i]),", tag:",string(t.tags[i]))
	//}

	//log.Error("texts:",len(t.texts),", tags:",len(t.tags))
	//printByteArray(t.texts)
	//log.Error("tags:",t.tags)

	for i := 0; i < n; i++ {
		ni, err := w.Write(t.texts[i])
		nn += int64(ni)
		if err != nil {
			return nn, err
		}

		ni, err = f(w, t.tags[i])
		nn += int64(ni)
		if err != nil {
			return nn, err
		}
		//fmt.Println(string(t.template))
	}
	ni, err := w.Write(t.texts[n])
	nn += int64(ni)
	return nn, err
}

// Execute substitutes template tags (placeholders) with the corresponding
// values from the map m and writes the result to the given writer w.
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// Returns the number of bytes written to w.
func (t *Template) Execute(w io.Writer, m map[string]interface{}) (int64, error) {
	return t.ExecuteFunc(w, func(w io.Writer, tag string) (int, error) { return stdTagFunc(w, tag, m) })
}

// ExecuteStd works the same way as Execute, but keeps the unknown placeholders.
// This can be used as a drop-in replacement for strings.Replacer
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// Returns the number of bytes written to w.
func (t *Template) ExecuteStd(w io.Writer, m map[string]interface{}) (int64, error) {
	return t.ExecuteFunc(w, func(w io.Writer, tag string) (int, error) { return keepUnknownTagFunc(w, t.startTag, t.endTag, tag, m) })
}

// ExecuteFuncString calls f on each template tag (placeholder) occurrence
// and substitutes it with the data written to TagFunc's w.
//
// Returns the resulting string.
//
// This function is optimized for frozen templates.
// Use ExecuteFuncString for constantly changing templates.
func (t *Template) ExecuteFuncString(f TagFunc) string {
	s, err := t.ExecuteFuncStringWithErr(f)
	if err != nil {
		panic(fmt.Sprintf("unexpected error: %s", err))
	}
	return s
}

func (t *Template) ExecuteFuncStringExtend(output io.Writer, f TagFunc) {
	err := t.ExecuteFuncStringWithErrExtend(output, f)
	if err != nil {
		panic(fmt.Sprintf("unexpected error: %s", err))
	}
}

// ExecuteFuncStringWithErr calls f on each template tag (placeholder) occurrence
// and substitutes it with the data written to TagFunc's w.
//
// Returns the resulting string.
//
// This function is optimized for frozen templates.
// Use ExecuteFuncString for constantly changing templates.
func (t *Template) ExecuteFuncStringWithErr(f TagFunc) (string, error) {
	//bb := templateBytesPool.Get()
	//defer templateBytesPool.Put(bb)
	bb := &bytebufferpool.ByteBuffer{}
	if _, err := t.ExecuteFunc(bb, f); err != nil {
		return "", err
	}
	s := string(bb.Bytes())
	//log.Error("s:",s)
	return s, nil
}

func (t *Template) ExecuteFuncStringWithErrExtend(output io.Writer, f TagFunc) error {
	if _, err := t.ExecuteFunc(output, f); err != nil {
		return err
	}
	return nil
}

// ExecuteString substitutes template tags (placeholders) with the corresponding
// values from the map m and returns the result.
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// This function is optimized for frozen templates.
// Use ExecuteString for constantly changing templates.
func (t *Template) ExecuteString(m map[string]interface{}) string {
	return t.ExecuteFuncString(func(w io.Writer, tag string) (int, error) { return stdTagFunc(w, tag, m) })
}

// ExecuteStringStd works the same way as ExecuteString, but keeps the unknown placeholders.
// This can be used as a drop-in replacement for strings.Replacer
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// This function is optimized for frozen templates.
// Use ExecuteStringStd for constantly changing templates.
func (t *Template) ExecuteStringStd(m map[string]interface{}) string {
	return t.ExecuteFuncString(func(w io.Writer, tag string) (int, error) { return keepUnknownTagFunc(w, t.startTag, t.endTag, tag, m) })
}

func stdTagFunc(w io.Writer, tag string, m map[string]interface{}) (int, error) {
	v := m[tag]
	if v == nil {
		return 0, nil
	}
	switch value := v.(type) {
	case []byte:
		return w.Write(value)
	case string:
		return w.Write([]byte(value))
	case TagFunc:
		return value(w, tag)
	default:
		panic(fmt.Sprintf("tag=%q contains unexpected value type=%#v. Expected []byte, string or TagFunc", tag, v))
	}
}

func keepUnknownTagFunc(w io.Writer, startTag, endTag, tag string, m map[string]interface{}) (int, error) {
	v, ok := m[tag]
	if !ok {
		if _, err := w.Write(unsafeString2Bytes(startTag)); err != nil {
			return 0, err
		}
		if _, err := w.Write(unsafeString2Bytes(tag)); err != nil {
			return 0, err
		}
		if _, err := w.Write(unsafeString2Bytes(endTag)); err != nil {
			return 0, err
		}
		return len(startTag) + len(tag) + len(endTag), nil
	}
	if v == nil {
		return 0, nil
	}
	switch value := v.(type) {
	case []byte:
		return w.Write(value)
	case string:
		return w.Write([]byte(value))
	case TagFunc:
		return value(w, tag)
	default:
		panic(fmt.Sprintf("tag=%q contains unexpected value type=%#v. Expected []byte, string or TagFunc", tag, v))
	}
}
