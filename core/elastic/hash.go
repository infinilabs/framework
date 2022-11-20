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

func GetShardIDWithRoutingOffset(esMajorVersion int,docID []byte, numberOfShards int, routingNumShards int, partitionOffset int) int {

	hash := int(murmur3Hash(docID))
	//esMajorVersion := 7 //only es after 7.0.0,need to consider routing hash to calculate hash
	if routingNumShards <= 0 {
		routingNumShards = getRoutingNumOfShards(numberOfShards, esMajorVersion) //number_of_routing_shards
	}

	if partitionOffset != 1 {
		partition := hash % partitionOffset
		hash = hash + partition
	}

	routingFactor := routingNumShards / numberOfShards

	var mod int
	if hash < 0 {
		newHash := routingNumShards - Abs(hash)
		mod = newHash % routingNumShards
		if mod < 0 {
			mod = mod + routingNumShards
		}
	} else {
		mod = hash % routingNumShards
	}
	shardID := mod / routingFactor
	return shardID
}

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}