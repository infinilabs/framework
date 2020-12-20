public class Hello {
    public static void main(String[] args) {
        if (args.length == 0) {
//            System.out.println(Murmur3HashFunction.hash("hell")==(0x5a0cb7c3));
//            System.out.println(Murmur3HashFunction.hash("hello")==(0xd7c31989));
//            System.out.println(Murmur3HashFunction.hash("hello w")==(0x22ab2984));
//            System.out.println(Murmur3HashFunction.hash("hello wo")==(0xdf0ca123));
//            System.out.println(Murmur3HashFunction.hash("hello wor")==(0xe7744d61));
//            System.out.println(Murmur3HashFunction.hash("The quick brown fox jumps over the lazy dog")==(0xe07db09c));
//            System.out.println(Murmur3HashFunction.hash("The quick brown fox jumps over the lazy cog")==(0x4e63d2ad));

            //        assertHash(0x5a0cb7c3, "hell");
//        assertHash(0xd7c31989, "hello");
//        assertHash(0x22ab2984, "hello w");
//        assertHash(0xdf0ca123, "hello wo");
//        assertHash(0xe7744d61, "hello wor");
//        assertHash(0xe07db09c, "The quick brown fox jumps over the lazy dog");
//        assertHash(0x4e63d2ad, "The quick brown fox jumps over the lazy cog");

//        Integer factor=getRoutingFactor(5,5);

//             String effectiveRouting;
            int factor=10;
             int partitionOffset=0;
            String effectiveRouting="2";
//            partitionOffset = 0;

            int shardId=calculateScaledShardId( effectiveRouting, partitionOffset,factor);
            System.out.println("factor:"+factor);
            System.out.println("shardId:"+shardId);
//            System.out.println(calculateScaledShardId(5, "_QmncXYBC53QmW9KV0Y6", partitionOffset,factor));
//            System.out.println(calculateScaledShardId(5, "UwmncXYBC53QmW9KgUef", partitionOffset,factor));
//
//            System.out.println(calculateScaledShardId(5, "BwmncXYBC53QmW9KZUdr", partitionOffset,factor));
//
//            System.out.println(calculateScaledShardId(5, "CwmncXYBC53QmW9KbkfU", partitionOffset,factor));
//
//            System.out.println(calculateScaledShardId(5, "TAmncXYBC53QmW9KeEfF", partitionOffset,factor));



//            System.out.println(Math.floorMod(Murmur3HashFunction.hash("dwhZcXYBC53QmW9KrpZT"),0));
//            System.out.println(Math.floorMod(Murmur3HashFunction.hash("dwhZcXYBC53QmW9KrpZT"),1));
//            System.out.println(Math.floorMod(Murmur3HashFunction.hash("dwhZcXYBC53QmW9KrpZT"),2));
//            System.out.println(Math.floorMod(Murmur3HashFunction.hash("dwhZcXYBC53QmW9KrpZT"),3));

//            System.out.println(Math.floorMod(Murmur3HashFunction.hash("2"),3));
//            System.out.println(Math.floorMod(Murmur3HashFunction.hash("3"),3));
//            System.out.println(Math.floorMod(Murmur3HashFunction.hash("HQhZcXYBC53QmW9KfpYh"),3));
//            System.out.println(Math.floorMod(Murmur3HashFunction.hash("MQhZcXYBC53QmW9KlJYu"),3));
//            System.out.println(Math.floorMod(Murmur3HashFunction.hash("bghZcXYBC53QmW9Kn5aM"),3));
//            System.out.println(Math.floorMod(Murmur3HashFunction.hash("hQhacXYBC53QmW9KXJcT"),3));
//
//            System.out.println(Math.floorMod(Murmur3HashFunction.hash("1"),3));
//            System.out.println(Math.floorMod(Murmur3HashFunction.hash("JghZcXYBC53QmW9KhJaH"),3));

        } else {
//            System.out.printf("hash: %s", Math.floorMod(Murmur3HashFunction.hash(args[0]),3));
        }
    }

    private static int calculateScaledShardId(String effectiveRouting, int partitionOffset,int routingFactor) {

        System.out.println(effectiveRouting);
        System.out.println(effectiveRouting.length());

        final int hash = Murmur3HashFunction.hash(effectiveRouting) + partitionOffset;

        System.out.println("Murmur3HashFunction.hash(effectiveRouting):"+Murmur3HashFunction.hash(effectiveRouting));
        System.out.println("partitionOffset:"+partitionOffset);
        System.out.println("effectiveRouting:"+effectiveRouting);
        System.out.println("hash:"+hash);

        int numShards=30; //TODO

        System.out.println("Math.floorMod(hash, numShards) :"+Math.floorMod(hash, numShards) );

        // we don't use IMD#getNumberOfShards since the index might have been shrunk such that we need to use the size
        // of original index to hash documents
        return Math.floorMod(hash, numShards) / routingFactor;
    }

    public static int getRoutingFactor(int sourceNumberOfShards, int targetNumberOfShards) {
        final int factor;
        if (sourceNumberOfShards < targetNumberOfShards) { // split
            factor = targetNumberOfShards / sourceNumberOfShards;
            if (factor * sourceNumberOfShards != targetNumberOfShards || factor <= 1) {
                throw new IllegalArgumentException("the number of source shards [" + sourceNumberOfShards + "] must be a " +
                        "factor of ["
                        + targetNumberOfShards + "]");
            }
        } else if (sourceNumberOfShards > targetNumberOfShards) { // shrink
            factor = sourceNumberOfShards / targetNumberOfShards;
            if (factor * targetNumberOfShards != sourceNumberOfShards || factor <= 1) {
                throw new IllegalArgumentException("the number of source shards [" + sourceNumberOfShards + "] must be a " +
                        "multiple of ["
                        + targetNumberOfShards + "]");
            }
        } else {
            factor = 1;
        }
        return factor;
    }

}
