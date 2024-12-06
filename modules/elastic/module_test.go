// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package elastic

import (
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/rubyniu105/framework/core/util"
	"testing"
)

func TestV7GetClusterStates(t *testing.T) {
	str := "{ \"_nodes\": { \"total\": 1, \"successful\": 1, \"failed\": 0 }, \"cluster_name\": \"es-v700\", \"cluster_uuid\": \"7NtDffC3RzGChhoOmgySig\", \"timestamp\": 1629611578327, \"status\": \"green\", \"indices\": { \"count\": 0, \"shards\": {}, \"docs\": { \"count\": 0, \"deleted\": 0 }, \"store\": { \"size_in_bytes\": 0 }, \"fielddata\": { \"memory_size_in_bytes\": 0, \"evictions\": 0 }, \"query_cache\": { \"memory_size_in_bytes\": 0, \"total_count\": 0, \"hit_count\": 0, \"miss_count\": 0, \"cache_size\": 0, \"cache_count\": 0, \"evictions\": 0 }, \"completion\": { \"size_in_bytes\": 0 }, \"segments\": { \"count\": 0, \"memory_in_bytes\": 0, \"terms_memory_in_bytes\": 0, \"stored_fields_memory_in_bytes\": 0, \"term_vectors_memory_in_bytes\": 0, \"norms_memory_in_bytes\": 0, \"points_memory_in_bytes\": 0, \"doc_values_memory_in_bytes\": 0, \"index_writer_memory_in_bytes\": 0, \"version_map_memory_in_bytes\": 0, \"fixed_bit_set_memory_in_bytes\": 0, \"max_unsafe_auto_id_timestamp\": -9223372036854776000, \"file_sizes\": {} } }, \"nodes\": { \"count\": { \"total\": 1, \"data\": 1, \"coordinating_only\": 0, \"master\": 1, \"ingest\": 1 }, \"versions\": [ \"7.0.0\" ], \"os\": { \"available_processors\": 24, \"allocated_processors\": 24, \"names\": [ { \"name\": \"Windows 10\", \"count\": 1 } ], \"pretty_names\": [ { \"pretty_name\": \"Windows 10\", \"count\": 1 } ], \"mem\": { \"total_in_bytes\": 137121308672, \"free_in_bytes\": 114813546496, \"used_in_bytes\": 22307762176, \"free_percent\": 84, \"used_percent\": 16 } }, \"process\": { \"cpu\": { \"percent\": 0 }, \"open_file_descriptors\": { \"min\": -1, \"max\": -1, \"avg\": 0 } }, \"jvm\": { \"max_uptime_in_millis\": 2021226, \"versions\": [ { \"version\": \"9.0.1.3\", \"vm_name\": \"OpenJDK 64-Bit Server VM\", \"vm_version\": \"9.0.1.3+11\", \"vm_vendor\": \"Azul Systems, Inc.\", \"bundled_jdk\": false, \"using_bundled_jdk\": null, \"count\": 1 } ], \"mem\": { \"heap_used_in_bytes\": 277003800, \"heap_max_in_bytes\": 1037959168 }, \"threads\": 66 }, \"fs\": { \"total_in_bytes\": 6000527532032, \"free_in_bytes\": 3111816585216, \"available_in_bytes\": 3111816585216 }, \"plugins\": [], \"network_types\": { \"transport_types\": { \"netty4\": 1 }, \"http_types\": { \"netty4\": 1 } }, \"discovery_types\": { \"zen\": 1 } } }"

	d1, err := jsonparser.GetInt(util.UnsafeStringToBytes(str), "indices", "segments", "max_unsafe_auto_id_timestamp")
	fmt.Println("xv:", d1, err)
	if err != nil {
		d, err := jsonparser.Set(util.UnsafeStringToBytes(str), []byte("-1"), "indices", "segments", "max_unsafe_auto_id_timestamp")
		if err == nil {
			str = util.UnsafeBytesToString(d)
		}
	}
	d1, err = jsonparser.GetInt(util.UnsafeStringToBytes(str), "indices", "segments", "max_unsafe_auto_id_timestamp")
	fmt.Println("xv:", d1, err)
	//xv,err:=jsonparser.GetInt([]byte(str),"indices.segments.max_unsafe_auto_id_timestamp")
	//fmt.Println("xv:",xv,err)
}
