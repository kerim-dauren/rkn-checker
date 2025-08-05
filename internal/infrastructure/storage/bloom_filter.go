package storage

import (
	"hash/fnv"
	"math"
)

type BloomFilter struct {
	bits []uint64
	size uint64
	k    uint64
}

func NewBloomFilter(n uint64, falsePositiveRate float64) *BloomFilter {
	if n == 0 {
		n = 1000000
	}
	if falsePositiveRate <= 0 || falsePositiveRate >= 1 {
		falsePositiveRate = 0.01
	}

	m := uint64(-float64(n) * math.Log(falsePositiveRate) / (math.Log(2) * math.Log(2)))
	k := uint64(float64(m) / float64(n) * math.Log(2))

	if k == 0 {
		k = 1
	}
	if k > 10 {
		k = 10
	}

	bitsArraySize := (m + 63) / 64

	return &BloomFilter{
		bits: make([]uint64, bitsArraySize),
		size: m,
		k:    k,
	}
}

func (bf *BloomFilter) Add(item string) {
	hashes := bf.getHashes(item)
	for i := uint64(0); i < bf.k; i++ {
		index := (hashes[0] + i*hashes[1]) % bf.size
		wordIndex := index / 64
		bitIndex := index % 64
		bf.bits[wordIndex] |= (1 << bitIndex)
	}
}

func (bf *BloomFilter) Contains(item string) bool {
	hashes := bf.getHashes(item)
	for i := uint64(0); i < bf.k; i++ {
		index := (hashes[0] + i*hashes[1]) % bf.size
		wordIndex := index / 64
		bitIndex := index % 64
		if (bf.bits[wordIndex] & (1 << bitIndex)) == 0 {
			return false
		}
	}
	return true
}

func (bf *BloomFilter) getHashes(item string) [2]uint64 {
	h1 := fnv.New64()
	h1.Write([]byte(item))
	hash1 := h1.Sum64()

	h2 := fnv.New64a()
	h2.Write([]byte(item))
	hash2 := h2.Sum64()

	return [2]uint64{hash1, hash2}
}

func (bf *BloomFilter) Clear() {
	for i := range bf.bits {
		bf.bits[i] = 0
	}
}

func (bf *BloomFilter) Size() uint64 {
	return bf.size
}

func (bf *BloomFilter) HashFunctions() uint64 {
	return bf.k
}

func (bf *BloomFilter) EstimatedFalsePositiveRate(itemCount uint64) float64 {
	if itemCount == 0 {
		return 0
	}
	return math.Pow(1-math.Exp(-float64(bf.k*itemCount)/float64(bf.size)), float64(bf.k))
}