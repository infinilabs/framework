package elastic

import (
	"infini.sh/framework/lib/murmurhash3"
	"math"
	"math/bits"
)

func getRoutingNumOfShards(numberOfShards int, esVersion int) int {
	if esVersion < 7 {
		return numberOfShards
	}

	log2MaxNumShards := 10
	ceil := math.Ceil(float64(bits.LeadingZeros32(uint32(numberOfShards - 1))))
	log2NumShards := 32 - ceil //Integer.numberOfLeadingZeros(numShards - 1); // ceil(logBase2(numShards))
	numSplits := log2MaxNumShards - int(log2NumShards)
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

	return murmurhash3.Murmur3A(newArray, 0)
}

func GetShardID(esMajorVersion int,docID []byte, numberOfShards int) int {
	return GetShardIDWithRoutingOffset(esMajorVersion,docID,numberOfShards,-1,1)
}

func GetShardIDWithRoutingOffset(esMajorVersion int,docID []byte, numberOfShards int, routingNumShards int, partitionOffset uint32) int {

	hash := murmur3Hash(docID)
	//esMajorVersion := 7 //only es after 7.0.0,need to consider routing hash to calculate hash
	if routingNumShards <= 0 {
		routingNumShards = getRoutingNumOfShards(numberOfShards, esMajorVersion) //number_of_routing_shards
	}

	if partitionOffset != 1 {
		partition := math.Mod(float64(hash), float64(partitionOffset))
		hash = hash + int32(partition)
	}

	routingFactor := routingNumShards / numberOfShards

	var mod int
	if hash < 0 {
		newHash := float64(routingNumShards) - math.Abs(float64(hash))
		mod = int(math.Mod(float64(newHash), float64(routingNumShards)))
		if mod < 0 {
			mod = mod + routingNumShards
		}
	} else {
		mod = int(math.Mod(float64(hash), float64(routingNumShards)))
	}

	shardID := int(mod) / routingFactor

	return shardID
}
