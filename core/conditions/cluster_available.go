package conditions

import (
	"fmt"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/errors"
)

type ClusterAvailable []string

func NewClusterAvailableCondition(names []string) (ClusterAvailable) {
	return ClusterAvailable(names)
}

func (c ClusterAvailable) Check(event ValuesMap) bool {
	for _, field := range c {
		cfg:=elastic.GetMetadata(field)
		if cfg==nil{
			panic(errors.Errorf("elasticsearch config [%v] not found, ",c))
		}
		if !cfg.IsAvailable(){
			return false
		}
	}
	return true
}

func (c ClusterAvailable) String() string {
	return fmt.Sprintf("cluster_available: %v", []string(c))
}

