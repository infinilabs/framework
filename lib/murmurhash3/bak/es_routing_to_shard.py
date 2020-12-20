# taken from https://gist.github.com/chris-belcher/1f1fee0b39f023d7bd04dc6e5b722359
# taken from https://stackoverflow.com/questions/13305290/is-there-a-pure-python-implementation-of-murmurhash
def murmur(data):

    seed = 0

    c1 = 0xcc9e2d51
    c2 = 0x1b873593

    length = len(data)
    h1 = seed
    roundedEnd = (length & 0xfffffffc)  # round down to 4 byte block
    for i in range(0, roundedEnd, 4):
        # little endian load order
        k1 = (ord(data[i]) & 0xff) | ((ord(data[i + 1]) & 0xff) << 8) | \
            ((ord(data[i + 2]) & 0xff) << 16) | (ord(data[i + 3]) << 24)
        k1 *= c1
        k1 = (k1 << 15) | ((k1 & 0xffffffff) >> 17)  # ROTL32(k1,15)
        k1 *= c2

        h1 ^= k1
        h1 = (h1 << 13) | ((h1 & 0xffffffff) >> 19)  # ROTL32(h1,13)
        h1 = h1 * 5 + 0xe6546b64

    # tail
    k1 = 0

    val = length & 0x03
    if val == 3:
        k1 = (ord(data[roundedEnd + 2]) & 0xff) << 16
    # fallthrough
    if val in [2, 3]:
        k1 |= (ord(data[roundedEnd + 1]) & 0xff) << 8
    # fallthrough
    if val in [1, 2, 3]:
        k1 |= ord(data[roundedEnd]) & 0xff
        k1 *= c1
        k1 = (k1 << 15) | ((k1 & 0xffffffff) >> 17)  # ROTL32(k1,15)
        k1 *= c2
        h1 ^= k1

    # finalization
    h1 ^= length

    # fmix(h1)
    h1 ^= ((h1 & 0xffffffff) >> 16)
    h1 *= 0x85ebca6b
    h1 ^= ((h1 & 0xffffffff) >> 13)
    h1 *= 0xc2b2ae35
    h1 ^= ((h1 & 0xffffffff) >> 16)

    return h1 & 0xffffffff


def hash(routing):
    return murmur(''.join(routing_to_bytes(routing)))


# https://github.com/elastic/elasticsearch/blob/2.4/core/src/main/java/org/elasticsearch/cluster/routing/Murmur3HashFunction.java
# Murmur3HashFunction.hash
def routing_to_bytes(routing):
    for i in routing:
        a = ord(i) % 256
        b = ord(i) >> 8
        assert ((a & 0xff) | ((b & 0xff) << 8)) == ord(i)
        yield chr(a)
        yield chr(b)


# https://github.com/elastic/elasticsearch/blob/2.4/core/src/main/java/org/elasticsearch/cluster/routing/OperationRouting.java
# generateShardId
def generate_shard_id(routing, nshards):
    return hash(routing) % nshards


# https://github.com/elastic/elasticsearch/blob/0f00c14afc8428a2a72c0b766d2171029dc8f6e1/core/src/test/java/org/elasticsearch/cluster/routing/operation/hash/murmur3/Murmur3HashFunctionTests.java
def test_hash():
    assert 0x5a0cb7c3 == hash("hell")
    assert 0xd7c31989 == hash("hello")
    assert 0x22ab2984 == hash("hello w")
    assert 0xdf0ca123 == hash("hello wo")
    assert 0xe7744d61 == hash("hello wor")
    assert 0xe07db09c == hash("The quick brown fox jumps over the lazy dog")
    assert 0x4e63d2ad == hash("The quick brown fox jumps over the lazy cog")


# https://github.com/elastic/elasticsearch/blob/d09d89f8c57c35146387268fc536d86b77d82435/core/src/test/java/org/elasticsearch/cluster/routing/OperationRoutingTests.java
def test_termtoshard():
    assert generate_shard_id("sEERfFzPSI", 8) == 1
    assert generate_shard_id("cNRiIrjzYd", 8) == 7
    assert generate_shard_id("BgfLBXUyWT", 8) == 5
    assert generate_shard_id("cnepjZhQnb", 8) == 3
    assert generate_shard_id("OKCmuYkeCK", 8) == 6
    assert generate_shard_id("OutXGRQUja", 8) == 5
    assert generate_shard_id("yCdyocKWou", 8) == 1
    assert generate_shard_id("KXuNWWNgVj", 8) == 2
    assert generate_shard_id("DGJOYrpESx", 8) == 4
    assert generate_shard_id("upLDybdTGs", 8) == 5
    assert generate_shard_id("yhZhzCPQby", 8) == 1
    assert generate_shard_id("EyCVeiCouA", 8) == 1
    assert generate_shard_id("tFyVdQauWR", 8) == 6
    assert generate_shard_id("nyeRYDnDQr", 8) == 6
    assert generate_shard_id("hswhrppvDH", 8) == 0
    assert generate_shard_id("BSiWvDOsNE", 8) == 5
    assert generate_shard_id("YHicpFBSaY", 8) == 1
    assert generate_shard_id("EquPtdKaBZ", 8) == 4
    assert generate_shard_id("rSjLZHCDfT", 8) == 5
    assert generate_shard_id("qoZALVcite", 8) == 7
    assert generate_shard_id("yDCCPVBiCm", 8) == 7
    assert generate_shard_id("ngizYtQgGK", 8) == 5
    assert generate_shard_id("FYQRIBcNqz", 8) == 0
    assert generate_shard_id("EBzEDAPODe", 8) == 2
    assert generate_shard_id("YePigbXgKb", 8) == 1
    assert generate_shard_id("PeGJjomyik", 8) == 3
    assert generate_shard_id("cyQIvDmyYD", 8) == 7
    assert generate_shard_id("yIEfZrYfRk", 8) == 5
    assert generate_shard_id("kblouyFUbu", 8) == 7
    assert generate_shard_id("xvIGbRiGJF", 8) == 3
    assert generate_shard_id("KWimwsREPf", 8) == 4
    assert generate_shard_id("wsNavvIcdk", 8) == 7
    assert generate_shard_id("xkWaPcCmpT", 8) == 0
    assert generate_shard_id("FKKTOnJMDy", 8) == 7
    assert generate_shard_id("RuLzobYixn", 8) == 2
    assert generate_shard_id("mFohLeFRvF", 8) == 4
    assert generate_shard_id("aAMXnamRJg", 8) == 7
    assert generate_shard_id("zKBMYJDmBI", 8) == 0
    assert generate_shard_id("ElSVuJQQuw", 8) == 7
    assert generate_shard_id("pezPtTQAAm", 8) == 7
    assert generate_shard_id("zBjjNEjAex", 8) == 2
    assert generate_shard_id("PGgHcLNPYX", 8) == 7
    assert generate_shard_id("hOkpeQqTDF", 8) == 3
    assert generate_shard_id("chZXraUPBH", 8) == 7
    assert generate_shard_id("FAIcSmmNXq", 8) == 5
    assert generate_shard_id("EZmDicyayC", 8) == 0
    assert generate_shard_id("GRIueBeIyL", 8) == 7
    assert generate_shard_id("qCChjGZYLp", 8) == 3
    assert generate_shard_id("IsSZQwwnUT", 8) == 3
    assert generate_shard_id("MGlxLFyyCK", 8) == 3
    assert generate_shard_id("YmscwrKSpB", 8) == 0
    assert generate_shard_id("czSljcjMop", 8) == 5
    assert generate_shard_id("XhfGWwNlng", 8) == 1
    assert generate_shard_id("cWpKJjlzgj", 8) == 7
    assert generate_shard_id("eDzIfMKbvk", 8) == 1
    assert generate_shard_id("WFFWYBfnTb", 8) == 0
    assert generate_shard_id("oDdHJxGxja", 8) == 7
    assert generate_shard_id("PDOQQqgIKE", 8) == 1
    assert generate_shard_id("bGEIEBLATe", 8) == 6
    assert generate_shard_id("xpRkJPWVpu", 8) == 2
    assert generate_shard_id("kTwZnPEeIi", 8) == 2
    assert generate_shard_id("DifcuqSsKk", 8) == 1
    assert generate_shard_id("CEmLmljpXe", 8) == 5
    assert generate_shard_id("cuNKtLtyJQ", 8) == 7
    assert generate_shard_id("yNjiAnxAmt", 8) == 5
    assert generate_shard_id("bVDJDCeaFm", 8) == 2
    assert generate_shard_id("vdnUhGLFtl", 8) == 0
    assert generate_shard_id("LnqSYezXbr", 8) == 5
    assert generate_shard_id("EzHgydDCSR", 8) == 3
    assert generate_shard_id("ZSKjhJlcpn", 8) == 1
    assert generate_shard_id("WRjUoZwtUz", 8) == 3
    assert generate_shard_id("RiBbcCdIgk", 8) == 4
    assert generate_shard_id("yizTqyjuDn", 8) == 4
    assert generate_shard_id("QnFjcpcZUT", 8) == 4
    assert generate_shard_id("agYhXYUUpl", 8) == 7
    assert generate_shard_id("UOjiTugjNC", 8) == 7
    assert generate_shard_id("nICGuWTdfV", 8) == 0
    assert generate_shard_id("NrnSmcnUVF", 8) == 2
    assert generate_shard_id("ZSzFcbpDqP", 8) == 3
    assert generate_shard_id("YOhahLSzzE", 8) == 5
    assert generate_shard_id("iWswCilUaT", 8) == 1
    assert generate_shard_id("zXAamKsRwj", 8) == 2
    assert generate_shard_id("aqGsrUPHFq", 8) == 5
    assert generate_shard_id("eDItImYWTS", 8) == 1
    assert generate_shard_id("JAYDZMRcpW", 8) == 4
    assert generate_shard_id("lmvAaEPflK", 8) == 7
    assert generate_shard_id("IKuOwPjKCx", 8) == 5
    assert generate_shard_id("schsINzlYB", 8) == 1
    assert generate_shard_id("OqbFNxrKrF", 8) == 2
    assert generate_shard_id("QrklDfvEJU", 8) == 6
    assert generate_shard_id("VLxKRKdLbx", 8) == 4
    assert generate_shard_id("imoydNTZhV", 8) == 1
    assert generate_shard_id("uFZyTyOMRO", 8) == 4
    assert generate_shard_id("nVAZVMPNNx", 8) == 3
    assert generate_shard_id("rPIdESYaAO", 8) == 5
    assert generate_shard_id("nbZWPWJsIM", 8) == 0
    assert generate_shard_id("wRZXPSoEgd", 8) == 3
    assert generate_shard_id("nGzpgwsSBc", 8) == 4
    assert generate_shard_id("AITyyoyLLs", 8) == 4
    print generate_shard_id("UwmncXYBC53QmW9KgUef", 5)

test_termtoshard()