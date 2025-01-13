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
	"bytes"
	"encoding/binary"
	"fmt"
	"hash"
	"math/bits"
)

type (
	murmurhash3A uint32
	murmurhash3C uint32
	murmurhash3F uint64
)

func NewMurmur3A() hash.Hash32 {
	var m murmurhash3A
	return &m
}

func (m *murmurhash3A) Reset() { *m = 0 }

func (m *murmurhash3A) Size() int {
	return 4
}

func (m *murmurhash3A) BlockSize() int {
	return 4
}

func (m *murmurhash3A) Write(p []byte) (n int, err error) {
	*m = murmurhash3A(Murmur3A(p, uint32(*m)))
	return len(p), nil
}

func (m *murmurhash3A) Sum32() uint32 {
	return uint32(*m)
}

func (m *murmurhash3A) Sum(in []byte) []byte {
	v := uint32(*m)
	return append(in, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func NewMurmur3C() hash.Hash32 {
	var m murmurhash3C
	return &m
}

func (m *murmurhash3C) Reset() { *m = 0 }

func (m *murmurhash3C) Size() int {
	return 4
}

func (m *murmurhash3C) BlockSize() int {
	return 16
}

func (m *murmurhash3C) Write(p []byte) (n int, err error) {
	*m = murmurhash3C(Murmur3C(p, uint32(*m))[0])
	return len(p), nil
}

func (m *murmurhash3C) Sum32() uint32 {
	return uint32(*m)
}

func (m *murmurhash3C) Sum(in []byte) []byte {
	v := uint32(*m)
	return append(in, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func NewMurmur3F() hash.Hash64 {
	var m murmurhash3F
	return &m
}

func (m *murmurhash3F) Reset() { *m = 0 }

func (m *murmurhash3F) Size() int {
	return 8
}

func (m *murmurhash3F) BlockSize() int {
	return 16
}

func (m *murmurhash3F) Write(p []byte) (n int, err error) {
	*m = murmurhash3F(Murmur3F(p, uint64(*m))[0])
	return len(p), nil
}

func (m *murmurhash3F) Sum64() uint64 {
	return uint64(*m)
}

func (m *murmurhash3F) Sum(in []byte) []byte {
	v := uint64(*m)
	return append(in, byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32), byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func rotl32(x uint32, r uint8) uint32 {
	return (x << r) | (x >> (32 - r))
}

func rotl64(x uint64, r uint8) uint64 {
	return (x << r) | (x >> (64 - r))
}

func fmix32(h uint32) uint32 {
	h ^= h >> 16
	h *= 0x85ebca6b
	h ^= h >> 13
	h *= 0xc2b2ae35
	h ^= h >> 16

	return h
}

func fmix64(h uint64) uint64 {
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33

	return h
}
func IntToByte(num int64) []byte {
	var buffer bytes.Buffer
	binary.Write(&buffer, binary.BigEndian, num)
	return buffer.Bytes()
}

func converToBianry(n int) string {
	return fmt.Sprintf("%b", n)
}

// MurmurHash3 for x86, 32-bit (MurmurHash3_x86_32)
func Murmur3A(key []byte, seed uint32) int32 {
	nblocks := len(key) / 4
	var h1 = int32(seed)

	var c1 uint32 = 0xcc9e2d51
	var c2 uint32 = 0x1b873593

	// body
	for i := 0; i < nblocks; i++ {
		bt := key[i*4:]
		if len(bt) <= 0 {
			continue
		}
		var k1 int32
		k1 = int32(binary.LittleEndian.Uint32(bt)) // TODO Validate

		k1 *= int32(c1)
		k1 = int32(bits.RotateLeft32(uint32(k1), 15))
		k1 *= int32(c2)
		h1 ^= k1
		h1 = int32(bits.RotateLeft32(uint32(h1), 13))

		h1 = h1 * 5
		h1 = int32(uint32(h1) + 0xe6546b64)

	}

	// tail
	var tail = key[nblocks*4:] // TODO Validate
	var k1 uint32
	switch len(key) & 3 {
	case 3:
		k1 ^= uint32(tail[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint32(tail[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint32(tail[0])
		k1 *= c1
		k1 = rotl32(k1, 15)
		k1 *= c2
		h1 ^= int32(k1)
	}

	//finalization
	h1 ^= int32(len(key))
	h1 = int32(fmix32(uint32(h1)))
	return int32(h1)
}

// MurmurHash3 for x86, 128-bit (MurmurHash3_x86_128)
func Murmur3C(key []byte, seed uint32) [4]uint32 {
	nblocks := len(key) / 16
	var h1 = seed
	var h2 = seed
	var h3 = seed
	var h4 = seed

	var c1 uint32 = 0x239b961b
	var c2 uint32 = 0xab0e9789
	var c3 uint32 = 0x38b34ae5
	var c4 uint32 = 0xa1e38b93

	// body
	for i := 0; i < nblocks; i++ {
		k1 := binary.LittleEndian.Uint32(key[(i*4+0)*4:]) // TODO Validate
		k2 := binary.LittleEndian.Uint32(key[(i*4+1)*4:])
		k3 := binary.LittleEndian.Uint32(key[(i*4+2)*4:])
		k4 := binary.LittleEndian.Uint32(key[(i*4+3)*4:])

		k1 *= c1
		k1 = rotl32(k1, 15)
		k1 *= c2
		h1 ^= k1

		h1 = rotl32(h1, 19)
		h1 += h2
		h1 = h1*5 + 0x561ccd1b

		k2 *= c2
		k2 = rotl32(k2, 16)
		k2 *= c3
		h2 ^= k2

		h2 = rotl32(h2, 17)
		h2 += h3
		h2 = h2*5 + 0x0bcaa747

		k3 *= c3
		k3 = rotl32(k3, 17)
		k3 *= c4
		h3 ^= k3

		h3 = rotl32(h3, 15)
		h3 += h4
		h3 = h3*5 + 0x96cd1c35

		k4 *= c4
		k4 = rotl32(k4, 18)
		k4 *= c1
		h4 ^= k4

		h4 = rotl32(h4, 13)
		h4 += h1
		h4 = h4*5 + 0x32ac3b17
	}

	// tail
	var tail = key[nblocks*16:] // TODO Validate
	var k1 uint32
	var k2 uint32
	var k3 uint32
	var k4 uint32
	switch len(key) & 15 {
	case 15:
		k4 ^= uint32(tail[14]) << 16
		fallthrough
	case 14:
		k4 ^= uint32(tail[13]) << 8
		fallthrough
	case 13:
		k4 ^= uint32(tail[12]) << 0
		k4 *= c4
		k4 = rotl32(k4, 18)
		k4 *= c1
		h4 ^= k4
		fallthrough
	case 12:
		k3 ^= uint32(tail[11]) << 24
		fallthrough
	case 11:
		k3 ^= uint32(tail[10]) << 16
		fallthrough
	case 10:
		k3 ^= uint32(tail[9]) << 8
		fallthrough
	case 9:
		k3 ^= uint32(tail[8]) << 0
		k3 *= c3
		k3 = rotl32(k3, 17)
		k3 *= c4
		h3 ^= k3
		fallthrough
	case 8:
		k2 ^= uint32(tail[7]) << 24
		fallthrough
	case 7:
		k2 ^= uint32(tail[6]) << 16
		fallthrough
	case 6:
		k2 ^= uint32(tail[5]) << 8
		fallthrough
	case 5:
		k2 ^= uint32(tail[4]) << 0
		k2 *= c2
		k2 = rotl32(k2, 16)
		k2 *= c3
		h2 ^= k2
		fallthrough
	case 4:
		k1 ^= uint32(tail[3]) << 24
		fallthrough
	case 3:
		k1 ^= uint32(tail[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint32(tail[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint32(tail[0]) << 0
		k1 *= c1
		k1 = rotl32(k1, 15)
		k1 *= c2
		h1 ^= k1
	}

	//finalization
	h1 ^= uint32(len(key))
	h2 ^= uint32(len(key))
	h3 ^= uint32(len(key))
	h4 ^= uint32(len(key))

	h1 += h2
	h1 += h3
	h1 += h4
	h2 += h1
	h3 += h1
	h4 += h1

	h1 = fmix32(h1)
	h2 = fmix32(h2)
	h3 = fmix32(h3)
	h4 = fmix32(h4)

	h1 += h2
	h1 += h3
	h1 += h4
	h2 += h1
	h3 += h1
	h4 += h1

	return [4]uint32{h1, h2, h3, h4}
}

// MurmurHash3 for x64, 128-bit (MurmurHash3_x64_128)
func Murmur3F(key []byte, seed uint64) [2]uint64 {
	nblocks := len(key) / 16
	var h1 = seed
	var h2 = seed

	var c1 uint64 = 0x87c37b91114253d5
	var c2 uint64 = 0x4cf5ad432745937f

	// body
	for i := 0; i < nblocks; i++ {
		k1 := binary.LittleEndian.Uint64(key[(i*2+0)*8:]) // TODO Validate
		k2 := binary.LittleEndian.Uint64(key[(i*2+1)*8:])

		k1 *= c1
		k1 = rotl64(k1, 31)
		k1 *= c2
		h1 ^= k1

		h1 = rotl64(h1, 27)
		h1 += h2
		h1 = h1*5 + 0x52dce729

		k2 *= c2
		k2 = rotl64(k2, 33)
		k2 *= c1
		h2 ^= k2

		h2 = rotl64(h2, 31)
		h2 += h1
		h2 = h2*5 + 0x38495ab5
	}

	// tail
	var tail = key[nblocks*16:] // TODO Validate
	var k1 uint64
	var k2 uint64
	switch len(key) & 15 {
	case 15:
		k2 ^= uint64(tail[14]) << 48
		fallthrough
	case 14:
		k2 ^= uint64(tail[13]) << 40
		fallthrough
	case 13:
		k2 ^= uint64(tail[12]) << 32
		fallthrough
	case 12:
		k2 ^= uint64(tail[11]) << 24
		fallthrough
	case 11:
		k2 ^= uint64(tail[10]) << 16
		fallthrough
	case 10:
		k2 ^= uint64(tail[9]) << 8
		fallthrough
	case 9:
		k2 ^= uint64(tail[8]) << 0
		k2 *= c2
		k2 = rotl64(k2, 33)
		k2 *= c1
		h2 ^= k2
		fallthrough
	case 8:
		k1 ^= uint64(tail[7]) << 56
		fallthrough
	case 7:
		k1 ^= uint64(tail[6]) << 48
		fallthrough
	case 6:
		k1 ^= uint64(tail[5]) << 40
		fallthrough
	case 5:
		k1 ^= uint64(tail[4]) << 32
		fallthrough
	case 4:
		k1 ^= uint64(tail[3]) << 24
		fallthrough
	case 3:
		k1 ^= uint64(tail[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint64(tail[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint64(tail[0]) << 0
		k1 *= c1
		k1 = rotl64(k1, 31)
		k1 *= c2
		h1 ^= k1
	}

	//finalization
	h1 ^= uint64(len(key))
	h2 ^= uint64(len(key))

	h1 += h2
	h2 += h1

	h1 = fmix64(h1)
	h2 = fmix64(h2)

	h1 += h2
	h2 += h1

	return [2]uint64{h1, h2}
}
