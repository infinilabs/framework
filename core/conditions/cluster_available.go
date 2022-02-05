package conditions

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
)

type ClusterAvailable []string

func NewClusterAvailableCondition(names []string) (ClusterAvailable) {
	return ClusterAvailable(names)
}

func (c ClusterAvailable) Check(event ValuesMap) bool {
	for _, field := range c {
		cfg:=elastic.GetMetadata(field)
		if cfg==nil{
			return false
		}
		if global.Env().IsDebug{
			log.Tracef("checking cluster [%v] health [%v]",field,cfg.IsAvailable())
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

