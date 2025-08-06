package storage

import (
	"fmt"
	"sync"
	"testing"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore()

	if store == nil {
		t.Fatal("NewMemoryStore() returned nil")
	}

	stats := store.Stats()
	if stats.TotalEntries != 0 {
		t.Errorf("NewMemoryStore() TotalEntries = %v, want 0", stats.TotalEntries)
	}
}

func TestMemoryStore_Update(t *testing.T) {
	store := NewMemoryStore()
	registry := createTestRegistry()

	err := store.Update(registry)
	if err != nil {
		t.Errorf("Update() unexpected error: %v", err)
	}

	stats := store.Stats()
	if stats.TotalEntries == 0 {
		t.Error("Update() did not update entry count")
	}
}

func TestMemoryStore_IsBlocked(t *testing.T) {
	store := NewMemoryStore()
	registry := createTestRegistry()
	store.Update(registry)

	tests := []struct {
		name          string
		normalizedURL string
		wantBlocked   bool
	}{
		{"blocked domain", "blocked.com", true},
		{"wildcard match", "sub.wildcard.com", true},
		{"blocked IP", "192.168.1.100", true},
		{"not blocked", "safe.com", false},
		{"empty URL", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := store.IsBlocked(tt.normalizedURL)

			if result == nil {
				t.Fatal("IsBlocked() returned nil")
			}

			if result.IsBlocked != tt.wantBlocked {
				t.Errorf("IsBlocked() = %v, want %v", result.IsBlocked, tt.wantBlocked)
			}
		})
	}
}

func TestMemoryStore_Concurrent(t *testing.T) {
	store := NewMemoryStore()
	registry := createLargeTestRegistry(10000)

	err := store.Update(registry)
	if err != nil {
		t.Fatalf("Update() failed: %v", err)
	}

	var wg sync.WaitGroup
	numWorkers := 100
	numOperations := 1000

	// Concurrent reads
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				url := generateTestURL(workerID, j)
				result := store.IsBlocked(url)
				if result == nil {
					t.Errorf("Worker %d: IsBlocked() returned nil for %s", workerID, url)
				}
			}
		}(i)
	}

	// Concurrent update
	wg.Add(1)
	go func() {
		defer wg.Done()
		newRegistry := createLargeTestRegistry(5000)
		err := store.Update(newRegistry)
		if err != nil {
			t.Errorf("Concurrent Update() failed: %v", err)
		}
	}()

	wg.Wait()
}

func TestMemoryStore_Clear(t *testing.T) {
	store := NewMemoryStore()
	registry := createTestRegistry()

	store.Update(registry)

	if store.Stats().TotalEntries == 0 {
		t.Error("Registry should have entries before clear")
	}

	store.Clear()

	stats := store.Stats()
	if stats.TotalEntries != 0 {
		t.Errorf("Clear() TotalEntries = %v, want 0", stats.TotalEntries)
	}
}

func BenchmarkMemoryStore_IsBlocked(b *testing.B) {
	store := NewMemoryStore()
	registry := createLargeTestRegistry(100000)
	store.Update(registry)

	testURLs := []string{
		"example.com",
		"blocked.com",
		"192.168.1.1",
		"sub.wildcard.com",
		"nonexistent.com",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			url := testURLs[i%len(testURLs)]
			store.IsBlocked(url)
			i++
		}
	})
}

func createTestRegistry() *domain.Registry {
	registry := domain.NewRegistry()

	// Add domain entries
	domainEntry, _ := domain.NewRegistryEntry(domain.BlockingTypeDomain, "blocked.com")
	registry.AddEntry(domainEntry)

	// Add wildcard entries
	wildcardEntry, _ := domain.NewRegistryEntry(domain.BlockingTypeWildcard, "*.wildcard.com")
	registry.AddEntry(wildcardEntry)

	// Add IP entries
	ipEntry, _ := domain.NewRegistryEntry(domain.BlockingTypeIP, "192.168.1.100")
	registry.AddEntry(ipEntry)

	return registry
}

func createLargeTestRegistry(size int) *domain.Registry {
	registry := domain.NewRegistry()

	for i := 0; i < size; i++ {
		var entry *domain.RegistryEntry
		var err error

		switch i % 4 {
		case 0:
			entry, err = domain.NewRegistryEntry(domain.BlockingTypeDomain, generateDomain(i))
		case 1:
			entry, err = domain.NewRegistryEntry(domain.BlockingTypeWildcard, "*."+generateDomain(i))
		case 2:
			entry, err = domain.NewRegistryEntry(domain.BlockingTypeIP, generateIP(i))
		case 3:
			entry, err = domain.NewRegistryEntry(domain.BlockingTypeURLPath, generateDomain(i)+"/blocked")
		}

		if err == nil {
			registry.AddEntry(entry)
		}
	}

	return registry
}

func generateDomain(i int) string {
	return fmt.Sprintf("domain%d.com", i)
}

func generateIP(i int) string {
	return fmt.Sprintf("192.168.%d.%d", i/256, i%256)
}

func generateTestURL(workerID, opID int) string {
	return fmt.Sprintf("test%d-%d.com", workerID, opID)
}
