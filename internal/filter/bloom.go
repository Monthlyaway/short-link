package filter

import (
	"sync"

	"github.com/bits-and-blooms/bloom/v3"
)

// BloomFilter wraps the bloom filter with thread-safety
type BloomFilter struct {
	filter *bloom.BloomFilter
	mu     sync.RWMutex
}

// NewBloomFilter creates a new Bloom filter with specified capacity and false positive rate
func NewBloomFilter(capacity uint, fpRate float64) *BloomFilter {
	return &BloomFilter{
		filter: bloom.NewWithEstimates(capacity, fpRate),
	}
}

// Add adds a short code to the Bloom filter
func (bf *BloomFilter) Add(shortCode string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	bf.filter.AddString(shortCode)
}

// Test checks if a short code might exist in the Bloom filter
// Returns true if the short code might exist (with possible false positives)
// Returns false if the short code definitely does not exist
func (bf *BloomFilter) Test(shortCode string) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	return bf.filter.TestString(shortCode)
}

// AddBatch adds multiple short codes to the Bloom filter
func (bf *BloomFilter) AddBatch(shortCodes []string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	for _, code := range shortCodes {
		bf.filter.AddString(code)
	}
}

// Clear clears the Bloom filter
func (bf *BloomFilter) Clear() {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	bf.filter.ClearAll()
}
