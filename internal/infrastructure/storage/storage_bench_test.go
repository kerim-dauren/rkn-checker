package storage

import (
	"fmt"
	"testing"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

func BenchmarkMemoryStore_IsBlocked_ExactMatch(b *testing.B) {
	store := NewMemoryStore()
	registry := createBenchmarkRegistry(100000)
	store.Update(registry)

	testURL := "blocked0.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.IsBlocked(testURL)
	}
}

func BenchmarkMemoryStore_IsBlocked_WildcardMatch(b *testing.B) {
	store := NewMemoryStore()
	registry := createBenchmarkRegistry(100000)
	store.Update(registry)

	testURL := "sub.wildcard0.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.IsBlocked(testURL)
	}
}

func BenchmarkMemoryStore_IsBlocked_NoMatch(b *testing.B) {
	store := NewMemoryStore()
	registry := createBenchmarkRegistry(100000)
	store.Update(registry)

	testURL := "nonexistent.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.IsBlocked(testURL)
	}
}

func BenchmarkRadixTree_Insert(b *testing.B) {
	tree := NewRadixTree()
	domains := generateBenchmarkDomains(b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Insert(domains[i], true)
	}
}

func BenchmarkRadixTree_Search(b *testing.B) {
	tree := NewRadixTree()
	domains := generateBenchmarkDomains(10000)

	for _, domain := range domains {
		tree.Insert(domain, true)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		domain := domains[i%len(domains)]
		tree.Search(domain)
	}
}

func BenchmarkRadixTree_MatchesWildcard(b *testing.B) {
	tree := NewRadixTree()

	for i := 0; i < 1000; i++ {
		tree.Insert(fmt.Sprintf("wildcard%d.com", i), true)
	}

	testDomain := "sub.wildcard500.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.MatchesWildcard(testDomain)
	}
}

func BenchmarkBloomFilter_Add(b *testing.B) {
	bloom := NewBloomFilter(1000000, 0.01)
	items := generateBenchmarkDomains(b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bloom.Add(items[i])
	}
}

func BenchmarkBloomFilter_Contains(b *testing.B) {
	bloom := NewBloomFilter(1000000, 0.01)
	items := generateBenchmarkDomains(100000)

	for _, item := range items {
		bloom.Add(item)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := items[i%len(items)]
		bloom.Contains(item)
	}
}

func createBenchmarkRegistry(size int) *domain.Registry {
	registry := domain.NewRegistry()

	for i := 0; i < size; i++ {
		var entry *domain.RegistryEntry
		var err error

		switch i % 4 {
		case 0:
			entry, err = domain.NewRegistryEntry(domain.BlockingTypeDomain, fmt.Sprintf("blocked%d.com", i))
		case 1:
			entry, err = domain.NewRegistryEntry(domain.BlockingTypeWildcard, fmt.Sprintf("*.wildcard%d.com", i))
		case 2:
			entry, err = domain.NewRegistryEntry(domain.BlockingTypeIP, fmt.Sprintf("192.%d.%d.%d", (i/65536)%256, (i/256)%256, i%256))
		case 3:
			entry, err = domain.NewRegistryEntry(domain.BlockingTypeURLPath, fmt.Sprintf("url%d.com/blocked", i))
		}

		if err == nil {
			registry.AddEntry(entry)
		}
	}

	return registry
}

func generateBenchmarkDomains(count int) []string {
	domains := make([]string, count)

	for i := 0; i < count; i++ {
		domains[i] = fmt.Sprintf("domain%d.com", i)
	}

	return domains
}
