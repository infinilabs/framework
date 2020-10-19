package fastjson_marshal

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestMarshal(t *testing.T) {
	m := map[string]interface{}{
		"string":  "foo",
		"uint":    uint(123),
		"uint8":   uint8(123),
		"uint16":  uint16(123),
		"uint32":  uint32(123),
		"uint64":  uint64(123),
		"int":     int(-123),
		"int8":    int8(-123),
		"int16":   int16(-123),
		"int32":   int32(-123),
		"int64":   int64(-123),
		"float32": float32(1.23),
		"float64": float64(-1.23),
		"bool":    true,
	}

	stdlibEncoding, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	var w Writer
	if err := Marshal(&w, m); err != nil {
		t.Fatal(err)
	}

	var fastjsonDecoded, stdlibDecoded interface{}
	mustUnmarshal(stdlibEncoding, &stdlibDecoded)
	mustUnmarshal(w.Bytes(), &fastjsonDecoded)
	if !reflect.DeepEqual(stdlibDecoded, fastjsonDecoded) {
		t.Fatal("different encoding")
	}
}

func TestMarshalNil(t *testing.T) {
	var w Writer
	Marshal(&w, nil)
	assertEncoded(t, &w, `null`)
}

func TestMarshalNilMap(t *testing.T) {
	var w Writer
	Marshal(&w, map[string]interface{}(nil))
	assertEncoded(t, &w, `null`)
}

func TestMarshalMarshaler(t *testing.T) {
	var w Writer
	Marshal(&w, marshalerFunc(func(w *Writer) error {
		w.String("custom_logic")
		return nil
	}))
	assertEncoded(t, &w, `"custom_logic"`)
}

func TestMarshalAppender(t *testing.T) {
	var w Writer
	Marshal(&w, appenderFunc(func(in []byte) []byte {
		return append(in, `"appended"`...)
	}))
	assertEncoded(t, &w, `"appended"`)
}

func TestMarshalStdlibMarshaler(t *testing.T) {
	var w Writer
	Marshal(&w, stdlibMarshalerFunc(func() ([]byte, error) {
		return []byte(`"json.Marshaled"`), nil
	}))
	assertEncoded(t, &w, `"json.Marshaled"`)
}

func TestMarshalStdlibMarshalerPanic(t *testing.T) {
	var w Writer
	err := Marshal(&w, stdlibMarshalerFunc(func() ([]byte, error) {
		panic("boom")
	}))
	assertEncoded(t, &w, `{"__PANIC__":"panic calling MarshalJSON for type fastjson.stdlibMarshalerFunc: boom"}`)
	expectedErr := `panic calling MarshalJSON for type fastjson.stdlibMarshalerFunc: boom`
	if err == nil || err.Error() != expectedErr {
		t.Fatalf("expected %q, got %q", expectedErr, err)
	}
}

func TestMarshalStdlibMarshalerError(t *testing.T) {
	var w Writer
	err := Marshal(&w, stdlibMarshalerFunc(func() ([]byte, error) {
		return nil, errors.New("boom")
	}))
	assertEncoded(t, &w, `{"__ERROR__":"json: error calling MarshalJSON for type fastjson.stdlibMarshalerFunc: boom"}`)
	expectedErr := `json: error calling MarshalJSON for type fastjson.stdlibMarshalerFunc: boom`
	if err == nil || err.Error() != expectedErr {
		t.Fatalf("expected %q, got %q", expectedErr, err)
	}
}

func TestMarshalMapValueError(t *testing.T) {
	var w Writer
	expectedErr := errors.New("nope")
	err := Marshal(&w, map[string]interface{}{
		"v": marshalerFunc(func(w *Writer) error {
			w.String("ERROR: nope")
			return expectedErr
		}),
	})
	assertEncoded(t, &w, `{"v":"ERROR: nope"}`)
	if err != expectedErr {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func mustUnmarshal(data []byte, out interface{}) {
	err := json.Unmarshal(data, out)
	if err != nil {
		panic(err)
	}
}

type marshalerFunc func(w *Writer) error

func (f marshalerFunc) MarshalFastJSON(w *Writer) error {
	return f(w)
}

type appenderFunc func([]byte) []byte

func (f appenderFunc) AppendJSON(in []byte) []byte {
	return f(in)
}

type stdlibMarshalerFunc func() ([]byte, error)

func (f stdlibMarshalerFunc) MarshalJSON() ([]byte, error) {
	return f()
}

func assertEncoded(t *testing.T, w *Writer, expected string) {
	actual := string(w.Bytes())
	if actual != expected {
		t.Fatalf("expected %q, got %q", expected, actual)
	}
}
