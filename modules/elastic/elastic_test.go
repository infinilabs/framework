package elastic

import (
	"fmt"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/adapter"
	"testing"
)

func TestGetShards(t *testing.T) {

	cfg:=elastic.ElasticsearchConfig{Endpoint: "http://192.168.3.201:9200",}

	api:=adapter.ESAPIV0{Config: cfg}
	shards,err:=api.GetIndices("*")
	//shards,err:=api.GetShards()
	fmt.Println(err)
	fmt.Println(util.ToJson((*shards),true))
}
