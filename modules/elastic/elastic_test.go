package elastic

import (
	"fmt"
	"infini.sh/framework/core/elastic/model"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/adapter"
	"src/github.com/buger/jsonparser"
	"testing"
)

func TestGetShards(t *testing.T) {

	cfg:= model.ElasticsearchConfig{Endpoint: "http://192.168.3.201:9200",}

	api:=adapter.ESAPIV0{Config: cfg}
	shards,err:=api.GetIndices("*")
	//shards,err:=api.GetShards()
	fmt.Println(err)
	fmt.Println(util.ToJson((*shards),true))
}


func TestV7GetClusterStates(t *testing.T) {
	str:="{ \"_nodes\": { \"total\": 1, \"successful\": 1, \"failed\": 0 }, \"cluster_name\": \"es-v700\", \"cluster_uuid\": \"7NtDffC3RzGChhoOmgySig\", \"timestamp\": 1629611578327, \"status\": \"green\", \"indices\": { \"count\": 0, \"shards\": {}, \"docs\": { \"count\": 0, \"deleted\": 0 }, \"store\": { \"size_in_bytes\": 0 }, \"fielddata\": { \"memory_size_in_bytes\": 0, \"evictions\": 0 }, \"query_cache\": { \"memory_size_in_bytes\": 0, \"total_count\": 0, \"hit_count\": 0, \"miss_count\": 0, \"cache_size\": 0, \"cache_count\": 0, \"evictions\": 0 }, \"completion\": { \"size_in_bytes\": 0 }, \"segments\": { \"count\": 0, \"memory_in_bytes\": 0, \"terms_memory_in_bytes\": 0, \"stored_fields_memory_in_bytes\": 0, \"term_vectors_memory_in_bytes\": 0, \"norms_memory_in_bytes\": 0, \"points_memory_in_bytes\": 0, \"doc_values_memory_in_bytes\": 0, \"index_writer_memory_in_bytes\": 0, \"version_map_memory_in_bytes\": 0, \"fixed_bit_set_memory_in_bytes\": 0, \"max_unsafe_auto_id_timestamp\": -9223372036854776000, \"file_sizes\": {} } }, \"nodes\": { \"count\": { \"total\": 1, \"data\": 1, \"coordinating_only\": 0, \"master\": 1, \"ingest\": 1 }, \"versions\": [ \"7.0.0\" ], \"os\": { \"available_processors\": 24, \"allocated_processors\": 24, \"names\": [ { \"name\": \"Windows 10\", \"count\": 1 } ], \"pretty_names\": [ { \"pretty_name\": \"Windows 10\", \"count\": 1 } ], \"mem\": { \"total_in_bytes\": 137121308672, \"free_in_bytes\": 114813546496, \"used_in_bytes\": 22307762176, \"free_percent\": 84, \"used_percent\": 16 } }, \"process\": { \"cpu\": { \"percent\": 0 }, \"open_file_descriptors\": { \"min\": -1, \"max\": -1, \"avg\": 0 } }, \"jvm\": { \"max_uptime_in_millis\": 2021226, \"versions\": [ { \"version\": \"9.0.1.3\", \"vm_name\": \"OpenJDK 64-Bit Server VM\", \"vm_version\": \"9.0.1.3+11\", \"vm_vendor\": \"Azul Systems, Inc.\", \"bundled_jdk\": false, \"using_bundled_jdk\": null, \"count\": 1 } ], \"mem\": { \"heap_used_in_bytes\": 277003800, \"heap_max_in_bytes\": 1037959168 }, \"threads\": 66 }, \"fs\": { \"total_in_bytes\": 6000527532032, \"free_in_bytes\": 3111816585216, \"available_in_bytes\": 3111816585216 }, \"plugins\": [], \"network_types\": { \"transport_types\": { \"netty4\": 1 }, \"http_types\": { \"netty4\": 1 } }, \"discovery_types\": { \"zen\": 1 } } }"

	d1,err:=jsonparser.GetInt(util.UnsafeStringToBytes(str),"indices","segments","max_unsafe_auto_id_timestamp")
	fmt.Println("xv:",d1,err)
	if err!=nil{
		d,err:=jsonparser.Set(util.UnsafeStringToBytes(str),[]byte("-1"),"indices","segments","max_unsafe_auto_id_timestamp")
		if err==nil{
			str=util.UnsafeBytesToString(d)
		}
	}
	d1,err=jsonparser.GetInt(util.UnsafeStringToBytes(str),"indices","segments","max_unsafe_auto_id_timestamp")
	fmt.Println("xv:",d1,err)
	//xv,err:=jsonparser.GetInt([]byte(str),"indices.segments.max_unsafe_auto_id_timestamp")
	//fmt.Println("xv:",xv,err)
}
