/*
End-User License Agreement (EULA) of INFINI SOFTWARE

This End-User License Agreement ("EULA") is a legal agreement between you and INFINI LIMITED

This EULA agreement governs your acquisition and use of our INFINI software ("Software") directly from INFINI LIMITED or indirectly through a INFINI LIMITED authorized reseller or distributor (a "Reseller").

Please read this EULA agreement carefully before completing the installation process and using the INFINI software. It provides a license to use the INFINI software and contains warranty information and liability disclaimers.

If you register for a free trial of the INFINI software, this EULA agreement will also govern that trial. By clicking "accept" or installing and/or using the INFINI software, you are confirming your acceptance of the Software and agreeing to become bound by the terms of this EULA agreement.

If you are entering into this EULA agreement on behalf of a company or other legal entity, you represent that you have the authority to bind such entity and its affiliates to these terms and conditions. If you do not have such authority or if you do not agree with the terms and conditions of this EULA agreement, do not install or use the Software, and you must not accept this EULA agreement.

This EULA agreement shall apply only to the Software supplied by INFINI LIMITED herewith regardless of whether other software is referred to or described herein. The terms also apply to any INFINI LIMITED updates, supplements, Internet-based services, and support services for the Software, unless other terms accompany those items on delivery. If so, those terms apply.

License Grant
INFINI LIMITED hereby grants you a personal, non-transferable, non-exclusive licence to use the INFINI software on your devices in accordance with the terms of this EULA agreement.

You are permitted to load the INFINI software (for example a PC, laptop, mobile or tablet) under your control. You are responsible for ensuring your device meets the minimum requirements of the INFINI software.

You are not permitted to:

Edit, alter, modify, adapt, translate or otherwise change the whole or any part of the Software nor permit the whole or any part of the Software to be combined with or become incorporated in any other software, nor decompile, disassemble or reverse engineer the Software or attempt to do any such things
Reproduce, copy, distribute, resell or otherwise use the Software for any commercial purpose
Allow any third party to use the Software on behalf of or for the benefit of any third party
Use the Software in any way which breaches any applicable local, national or international law
use the Software for any purpose that INFINI LIMITED considers is a breach of this EULA agreement
Intellectual Property and Ownership
INFINI LIMITED shall at all times retain ownership of the Software as originally downloaded by you and all subsequent downloads of the Software by you. The Software (and the copyright, and other intellectual property rights of whatever nature in the Software, including any modifications made thereto) are and shall remain the property of INFINI LIMITED.

INFINI LIMITED reserves the right to grant licences to use the Software to third parties.

Termination
This EULA agreement is effective from the date you first use the Software and shall continue until terminated. You may terminate it at any time upon written notice to INFINI LIMITED.

It will also terminate immediately if you fail to comply with any term of this EULA agreement. Upon such termination, the licenses granted by this EULA agreement will immediately terminate and you agree to stop all access and use of the Software. The provisions that by their nature continue and survive will survive any termination of this EULA agreement.

Governing Law
This EULA agreement, and any dispute arising out of or in connection with this EULA agreement, shall be governed by and construed in accordance with the laws of cn.
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
