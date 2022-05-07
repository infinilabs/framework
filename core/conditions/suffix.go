package conditions

import (
	"errors"
	"fmt"
	"infini.sh/framework/core/util"
)

// Suffix is a Condition for checking field if the field whether end with specify string or not.
type Suffix struct {
	Field string
	Data string
}

func NewSuffixCondition(fields map[string]interface{}) (hasFieldsCondition Suffix, err error) {
	c:= Suffix{}
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
func (c Suffix) Check(event ValuesMap) bool {
	value, err := event.GetValue(c.Field)
	if err != nil {
		return false
	}
	str,ok:=value.(string)
	if ok{
		if util.SuffixStr(str,c.Data){
			return true
		}
	}
	return false
}

func (c Suffix) String() string {
	return fmt.Sprintf("field: %v suffix: %v", c.Field,c.Field)
}
