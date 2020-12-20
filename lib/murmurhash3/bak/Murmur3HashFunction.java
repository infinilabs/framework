//public void testKnownValues() {
//        assertHash(0x5a0cb7c3, "hell");
//        assertHash(0xd7c31989, "hello");
//        assertHash(0x22ab2984, "hello w");
//        assertHash(0xdf0ca123, "hello wo");
//        assertHash(0xe7744d61, "hello wor");
//        assertHash(0xe07db09c, "The quick brown fox jumps over the lazy dog");
//        assertHash(0x4e63d2ad, "The quick brown fox jumps over the lazy cog");
//        }
//
//private static void assertHash(int expected, String stringInput) {
//        assertEquals(expected, Murmur3HashFunction.hash(stringInput));
//        }

import java.util.*;

public final class Murmur3HashFunction {

    private Murmur3HashFunction() {
        //no instance
    }

    public static byte[] strToByteArray(String str) {
        if (str == null) {
            return null;
        }
        byte[] byteArray = str.getBytes();
        return byteArray;
    }

    public static int hash(String routing) {

        var bx=strToByteArray(routing);
        System.out.println(Arrays.toString(bx));

        final byte[] bytesToHash = new byte[routing.length() * 2];
        for (int i = 0; i < routing.length(); ++i) {
            final char c = routing.charAt(i);
            final byte b1 = (byte) c, b2 = (byte) (c >>> 8);
            assert ((b1 & 0xFF) | ((b2 & 0xFF) << 8)) == c; // no information loss
            bytesToHash[i * 2] = b1;
            bytesToHash[i * 2 + 1] = b2;
        }

        System.out.println(Arrays.toString(bytesToHash));

        return hash(bytesToHash, 0, bytesToHash.length);
    }

    public static int hash(byte[] bytes, int offset, int length) {
        return murmurhash3_x86_32(bytes, offset, length, 0);
    }
    public static String toBinary(int value) {
        return Integer.toBinaryString(value); //0x20 | 这个是为了保证这个string长度是6位数
    }
    /** Returns the MurmurHash3_x86_32 hash.
     * Original source/tests at https://github.com/yonik/java_util/
     */
    @SuppressWarnings("fallthrough")
    private  static int murmurhash3_x86_32(byte[] data, int offset, int len, int seed) {

        System.out.println(":"+offset+":"+len+":"+seed);

        final int c1 = 0xcc9e2d51;
        final int c2 = 0x1b873593;

        int h1 = seed;
        int roundedEnd = offset + (len & 0xfffffffc);  // round down to 4 byte block

        System.out.println("seed:"+seed);
        System.out.println("nblocs:"+roundedEnd);

        for (int i=offset; i<roundedEnd; i+=4) {
            System.out.println("\n"+i);

            // little endian load order
            int k1 = (data[i] & 0xff) | ((data[i+1] & 0xff) << 8) | ((data[i+2] & 0xff) << 16) | (data[i+3] << 24);
            System.out.println("k1:"+k1+","+toBinary(k1));

            k1 *= c1;
            System.out.println("k1*c1:"+k1+","+toBinary(k1));

            k1 = Integer.rotateLeft(k1, 15);
            k1 *= c2;
            System.out.println("k1*c2:"+k1+","+toBinary(k1));

//            System.out.println(i+",k1*c1 for:"+k1+","+toBinary(k1,4));

            System.out.println("h1:"+h1+","+toBinary(h1));
            h1 ^= k1;
            h1 = Integer.rotateLeft(h1, 13);
            System.out.println("h1 after:"+h1+","+toBinary(h1));
            h1 = h1*5+0xe6546b64;
            System.out.println("h1 after:"+h1+","+toBinary(h1));
        }
        System.out.println("after for:"+h1);

        // tail
        int k1 = 0;

        switch(len & 0x03) {
            case 3:
                k1 = (data[roundedEnd + 2] & 0xff) << 16;
                // fallthrough
            case 2:
                k1 |= (data[roundedEnd + 1] & 0xff) << 8;
                // fallthrough
            case 1:
                k1 |= (data[roundedEnd] & 0xff);
                k1 *= c1;
                k1 = Integer.rotateLeft(k1, 15);
                k1 *= c2;
                h1 ^= k1;
        }

        // finalization
        h1 ^= len;


        // fmix(h1);
        h1 ^= h1 >>> 16;
        h1 *= 0x85ebca6b;
        h1 ^= h1 >>> 13;
        h1 *= 0xc2b2ae35;
        h1 ^= h1 >>> 16;
        System.out.println(h1);

        return h1;
    }
}

