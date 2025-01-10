package yaml

import (
	"io"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// QuotedKey 用于存储键的原始值和解析后的值。
type QuotedKey struct {
	Value         string // 解析后的键值 (不带引号)
	OriginalValue string // 原始的键值 (带引号)
}

// CustomDecoder 是一个自定义的解码器。
type CustomDecoder struct {
	decoder *yaml.Decoder
}

// NewCustomDecoder 创建一个新的 CustomDecoder 实例。
func NewCustomDecoder(r io.Reader) *CustomDecoder {
	return &CustomDecoder{
		decoder: yaml.NewDecoder(r),
	}
}

// Decode 解码 YAML 数据，并在处理 map 键时保留原始引号样式。
func (cd *CustomDecoder) Decode(v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &yaml.TypeError{Errors: []string{"Decode target must be a non-nil pointer"}}
	}

	// 解码到中间 map
	var intermediate interface{}
	if err := cd.decoder.Decode(&intermediate); err != nil {
		return err
	}

	// 使用反射处理 map 键
	processedValue := cd.processValue(reflect.ValueOf(intermediate)).Interface()

	// 将处理后的值设置到目标变量
	rv.Elem().Set(reflect.ValueOf(processedValue))
	return nil
}

// processValue 递归地处理值，保留 map 键的原始引号样式。
func (cd *CustomDecoder) processValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Map:
		return cd.processMap(value)
	case reflect.Slice:
		return cd.processSlice(value)
	default:
		return value
	}
}

// processMap 处理 map 类型的值，保留键的原始引号样式。
func (cd *CustomDecoder) processMap(value reflect.Value) reflect.Value {
	newMap := reflect.MakeMap(value.Type())
	for _, k := range value.MapKeys() {
		v := value.MapIndex(k)

		// 处理键
		newKey := cd.processKey(k)

		// 递归处理值
		newValue := cd.processValue(v)

		newMap.SetMapIndex(newKey, newValue)
	}
	return newMap
}

// processSlice 处理切片类型的值。
func (cd *CustomDecoder) processSlice(value reflect.Value) reflect.Value {
	newSlice := reflect.MakeSlice(value.Type(), value.Len(), value.Cap())
	for i := 0; i < value.Len(); i++ {
		elem := value.Index(i)
		newElem := cd.processValue(elem)
		newSlice.Index(i).Set(newElem)
	}
	return newSlice
}

// processKey 处理键，如果是带引号的字符串，则转换为 QuotedKey 类型。
func (cd *CustomDecoder) processKey(key reflect.Value) reflect.Value {
	if key.Kind() == reflect.String {
		strKey := key.String()
		if (strings.HasPrefix(strKey, `"`) && strings.HasSuffix(strKey, `"`)) ||
			(strings.HasPrefix(strKey, `'`) && strings.HasSuffix(strKey, `'`)) {
			quotedKey := QuotedKey{
				Value:         strings.Trim(strKey, `"'`),
				OriginalValue: strKey,
			}
			return reflect.ValueOf(quotedKey)
		}
	}
	return key
}
