package conditions

import (
	"errors"
	"fmt"
	"infini.sh/framework/core/util"
)

// Prefix is a Condition for checking field is prefix with some specify string.
type Prefix struct {
	Field string
	Data string
}

func NewPrefixCondition(fields map[string]interface{}) (hasFieldsCondition Prefix, err error) {
	c:= Prefix{}
	if len(fields)==1{
		for field, value := range util.MapStr(fields).Flatten() {
			c.Field=field
			var ok bool
			c.Data,ok=value.(string)
			if !ok{
				return c, errors.New("invalid in parameters")
			}
			break
		}
	}else{
		return c, errors.New("invalid in parameters")
	}
	return c,nil
}

// Check determines whether the given event matches this condition
func (c Prefix) Check(event ValuesMap) bool {
	value, err := event.GetValue(c.Field)
	if err != nil {
		return false
	}
	str,ok:=value.(string)
	if ok{
		if util.PrefixStr(str,c.Data){
			return true
		}
	}
	return false
}

func (c Prefix) String() string {
	return fmt.Sprintf("field: %v prefix: %v", c.Field,c.Field)
}
