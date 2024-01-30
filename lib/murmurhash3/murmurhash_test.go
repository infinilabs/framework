/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package murmurhash3

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"math"
	"math/bits"

	"testing"
)

func getRoutingNumOfShards(numberOfShards int, esVersion int) int {

	///Users/medcl/Documents/elasticsearch/server/src/main/java/org/elasticsearch/cluster/metadata/MetadataCreateIndexService.java

	//if (indexVersionCreated.onOrAfter(Version.V_7_0_0)) {
	//	// only select this automatically for indices that are created on or after 7.0 this will prevent this new behaviour
	//	// until we have a fully upgraded cluster. Additionally it will make integratin testing easier since mixed clusters
	//	// will always have the behavior of the min node in the cluster.
	//	//
	//	// We use as a default number of routing shards the higher number that can be expressed
	//	// as {@code numShards * 2^x`} that is less than or equal to the maximum number of shards: 1024.
	//	int log2MaxNumShards = 10; // logBase2(1024)
	//	int log2NumShards = 32 - Integer.numberOfLeadingZeros(numShards - 1); // ceil(logBase2(numShards))
	//	int numSplits = log2MaxNumShards - log2NumShards;
	//	numSplits = Math.max(1, numSplits); // Ensure the index can be split at least once
	//	return numShards * 1 << numSplits;
	//} else {
	//	return numShards;
	//}

	if esVersion < 7 {
		return numberOfShards
	}

	log2MaxNumShards := 10
	ceil := math.Ceil(float64(bits.LeadingZeros32(uint32(numberOfShards - 1))))
	log2NumShards := 32 - ceil //Integer.numberOfLeadingZeros(numShards - 1); // ceil(logBase2(numShards))
	//fmt.Println("math.Ceil(math.Logb(float64(numberOfShards-1)):",ceil)
	//fmt.Println("log2NumShards",log2NumShards)
	numSplits := log2MaxNumShards - int(log2NumShards)
	//fmt.Println("numSplits",numSplits)
	numSplits = int(math.Max(1, float64(numSplits)))
	return numberOfShards * 1 << numSplits
}

func murmur3Hash(data []byte) int32 {
	newArray := make([]byte, len(data)*2)
	j := 0
	for _, v := range data {
		newArray[j] = v
		j = j + 2
	}

	return Murmur3A(newArray, 0)
}

var debugMurmur3Hash = false
//另外，指定routing还有个弊端就是容易造成负载不均衡。所以ES提供了一种机制可以将数据路由到一组shard上面，而不是某一个。只需在创建索引时（也只能在创建时）设置index.routing_partition_size，默认值是1，即只路由到1个shard，可以将其设置为大于1且小于索引shard总数的某个值，就可以路由到一组shard了。值越大，数据越均匀。当然，从设置就能看出来，这个设置是针对单个索引的，可以加入到动态模板中，以对多个索引生效。指定后，shard的计算方式变为：
//shard_num = (hash(_routing) + hash(_id) % routing_partition_size) % num_primary_shards
//对于同一个routing值，hash(_routing)的结果固定的，hash(_id) % routing_partition_size的结果有 routing_partition_size 个可能的值，两个组合在一起，对于同一个routing值的不同doc，也就能计算出 routing_partition_size 可能的shard num了，即一个shard集合。但要注意这样做以后有两个限制：
func getShardID(docID []byte, numberOfShards int, routingNumShards int, partitionOffset uint32) int {

	hash := murmur3Hash(docID)
	if debugMurmur3Hash {
		fmt.Println("murmur3Hash(docID):", hash)
	}

	esMajorVersion := 7 //only es after 7.0.0,need to consider routing hash to calculate hash
	if routingNumShards <= 0 {
		routingNumShards = getRoutingNumOfShards(numberOfShards, esMajorVersion) //number_of_routing_shards
	}

	if debugMurmur3Hash {
		fmt.Println("routingNumShards:", routingNumShards)
	}

	//index.routing_partition_size=1
	if partitionOffset != 1 {
		partition := math.Mod(float64(hash), float64(partitionOffset))

		if debugMurmur3Hash {
			fmt.Println("partition:", partition)
		}

		hash = hash + int32(partition)
		if debugMurmur3Hash {
			fmt.Println("partition:", partition)
			fmt.Println("hash + int32(partition):", hash)
		}
	}

	routingFactor := routingNumShards / numberOfShards

	if debugMurmur3Hash {
		fmt.Println("routingFactor(routingNumShards/numberOfShards):", routingFactor)
	}

	var mod int
	if hash < 0 {
		//int i = (n < 0) ? (m - (abs(n) % m) ) %m : (n % m);
		newHash := float64(routingNumShards) - math.Abs(float64(hash))
		mod = int(math.Mod(float64(newHash), float64(routingNumShards)))
		if mod < 0 {
			mod = mod + routingNumShards
		}
	} else {
		mod = int(math.Mod(float64(hash), float64(routingNumShards)))
	}

	if debugMurmur3Hash {
		fmt.Println("mod:", mod)
	}

	shardID := int(mod) / routingFactor
	//fmt.Println("shardID:",shardID)

	return shardID
}

func TestHash(test *testing.T) {
	//str:="UwmncXYBC53QmW9KgUef" //hash:706800888
	//arr:=[]byte(str)
	//fmt.Println([]byte(str))
	//
	//newArray:=make([]byte,len(arr)*2)
	//j:=0;
	//for _,v:=range arr{
	//	newArray[j]=v
	//	j=j+2
	//}
	//fmt.Println(newArray)
	//h32:=murmurhash3.Murmur3A(newArray,0)
	//fmt.Println("hash",int(h32))
	//
	//esMajorVersion:=7 //only es after 7.0.0,need to consider routing hash to calculate hash
	//numberOfShards:=5
	////routingFactor:=128
	//
	//
	////only after 7.0, /elasticsearch/server/src/main/java/org/elasticsearch/cluster/metadata/MetadataCreateIndexService.java
	////routingNumShards = calculateNumRoutingShards(numTargetShards, indexVersionCreated);
	//
	//
	////routingNumShards:=routingFactor*numberOfShards//640 //TODO, come from 5
	//routingNumShards:=getRoutingNumOfShards(numberOfShards ,esMajorVersion ) //number_of_routing_shards
	//fmt.Println("routingNumShards:",routingNumShards)
	//
	////index.routing_partition_size=1
	//
	//routingFactor:=routingNumShards/numberOfShards
	//
	//mod:= math.Mod(float64(h32), float64(routingNumShards))
	//fmt.Println("mod(hash,shards):",mod)
	//fmt.Println("routingFactor:",routingFactor)
	//
	//
	////numberOfShards * routingFactor == routingNumShards
	//
	//shardID:=int(mod)/routingFactor
	//fmt.Println("shardID:",shardID)
	//
	////h32 := murmur3.Sum32WithSeed([]byte(str),0)

	//DELETE medcl
	//PUT medcl
	//{
	//	"settings": {
	//	"number_of_shards": 5,
	//		"number_of_replicas": 0
	//}
	//}
	//POST medcl/_doc/1
	//{"doc":""}
	//POST medcl/_doc/2
	//{"doc":""}
	//POST medcl/_doc/3
	//{"doc":""}
	//POST medcl/_doc/4
	//{"doc":""}
	//POST medcl/_doc/5
	//{"doc":""}
	//POST medcl/_doc/6
	//{"doc":""}
	//POST medcl/_doc/7
	//{"doc":""}
	//POST medcl/_doc/8
	//{"doc":""}
	assert.Equal(test, getShardID([]byte("3"), 5, -1, 1), 0)
	assert.Equal(test, getShardID([]byte("5"), 5, -1, 1), 0)
	assert.Equal(test, getShardID([]byte("4"), 5, -1, 1), 1)
	assert.Equal(test, getShardID([]byte("8"), 5, -1, 1), 2)
	assert.Equal(test, getShardID([]byte("7"), 5, -1, 1), 2)
	assert.Equal(test, getShardID([]byte("2"), 5, -1, 1), 3)
	assert.Equal(test, getShardID([]byte("6"), 5, -1, 1), 3)
	assert.Equal(test, getShardID([]byte("1"), 5, -1, 1), 4)

	//curl -XPUT "http://localhost:9200/medcl1" -H 'Content-Type: application/json' -d'{  "settings": {    "number_of_shards": 5,"number_of_replicas": 0  }}' -u "elastic:password"
	//curl -XPOST "http://localhost:9200/medcl1/_doc/UwmncXYBC53QmW9KgUef" -H 'Content-Type: application/json' -d'{  "a":1}' -u "elastic:password"
	assert.Equal(test, getShardID([]byte("_QmncXYBC53QmW9KV0Y6"), 5, -1, 1), 1)
	assert.Equal(test, getShardID([]byte("UwmncXYBC53QmW9KgUef"), 5, -1, 1), 1)
	assert.Equal(test, getShardID([]byte("BwmncXYBC53QmW9KZUdr"), 5, -1, 1), 2)
	assert.Equal(test, getShardID([]byte("CwmncXYBC53QmW9KbkfU"), 5, -1, 1), 3)
	assert.Equal(test, getShardID([]byte("TAmncXYBC53QmW9KeEfF"), 5, -1, 1), 4)

	//PUT medcl
	//{
	//	"settings": {
	//	"number_of_shards": 3,
	//		"number_of_replicas": 0
	//}
	//}
	assert.Equal(test, getShardID([]byte("1"), 3, -1, 1), 2)
	assert.Equal(test, getShardID([]byte("2"), 3, -1, 1), 1)
	assert.Equal(test, getShardID([]byte("3"), 3, -1, 1), 1)
	assert.Equal(test, getShardID([]byte("4"), 3, -1, 1), 1)
	assert.Equal(test, getShardID([]byte("5"), 3, -1, 1), 0)
	assert.Equal(test, getShardID([]byte("6"), 3, -1, 1), 2)
	assert.Equal(test, getShardID([]byte("7"), 3, -1, 1), 0)
	assert.Equal(test, getShardID([]byte("8"), 3, -1, 1), 2)

	//PUT medcl
	//{
	//	"settings": {
	//	"number_of_shards": 3,
	//		"number_of_replicas": 0,
	//		"number_of_routing_shards":30
	//}
	//}
	assert.Equal(test, getShardID([]byte("1"), 3, 30, 1), 2)
	assert.Equal(test, getShardID([]byte("2"), 3, 30, 1), 2)
	assert.Equal(test, getShardID([]byte("3"), 3, 30, 1), 1)
	assert.Equal(test, getShardID([]byte("4"), 3, 30, 1), 2)
	assert.Equal(test, getShardID([]byte("5"), 3, 30, 1), 2)
	assert.Equal(test, getShardID([]byte("6"), 3, 30, 1), 2)
	assert.Equal(test, getShardID([]byte("7"), 3, 30, 1), 1)
	assert.Equal(test, getShardID([]byte("8"), 3, 30, 1), 1)

	//DELETE medcl344
	//PUT medcl344
	//{
	//	"settings": {
	//	"number_of_shards": 3,
	//		"number_of_replicas": 0
	//}
	//}
	//PUT medcl344/_doc/1?routing=a&refresh=true
	//{
	//	"title": "This is a document"
	//}
	//
	//PUT medcl344/_doc/2?routing=a&refresh=true
	//{
	//	"title": "This is a document"
	//}
	//
	//PUT medcl344/_doc/3?routing=a&refresh=true
	//{
	//	"title": "This is a document"
	//}
	//
	//PUT medcl344/_doc/4?routing=b&refresh=true
	//{
	//	"title": "This is a document"
	//}
	//
	//PUT medcl344/_doc/5?routing=b&refresh=true
	//{
	//	"title": "This is a document"
	//}
	//GET medcl344/_search?preference=_shards:0&stored_fields=doc
	//assert.Equal(test,getShardID([]byte("1"),3,0,1),0)
	//assert.Equal(test,getShardID([]byte("2"),3,0,1),0)
	//assert.Equal(test,getShardID([]byte("3"),3,0,1),0)
	//assert.Equal(test,getShardID([]byte("4"),3,0,1),2)
	//assert.Equal(test,getShardID([]byte("5"),3,0,1),2)
}
