package storage

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

func TestMemoryStore_ConcurrentReads(t *testing.T) {
	store := NewMemoryStore()
	registry := createLargeConcurrentRegistry(50000)

	err := store.Update(registry)
	if err != nil {
		t.Fatalf("Failed to update store: %v", err)
	}

	const (
		numWorkers = 100
		numReads   = 1000
	)

	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < numReads; j++ {
				url := fmt.Sprintf("blocked%d.com", (workerID*numReads+j)%50000)
				result := store.IsBlocked(url)

				if result != nil {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&errorCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	totalOps := int64(numWorkers * numReads)
	if atomic.LoadInt64(&successCount) != totalOps {
		t.Errorf("Expected %d successful reads, got %d", totalOps, atomic.LoadInt64(&successCount))
	}

	if atomic.LoadInt64(&errorCount) > 0 {
		t.Errorf("Got %d errors during concurrent reads", atomic.LoadInt64(&errorCount))
	}
}

func TestMemoryStore_ConcurrentReadsAndWrites(t *testing.T) {
	store := NewMemoryStore()
	initialRegistry := createLargeConcurrentRegistry(10000)

	err := store.Update(initialRegistry)
	if err != nil {
		t.Fatalf("Failed to update store: %v", err)
	}

	const (
		numReaders = 50
		numWriters = 5
		numOps     = 1000
		duration   = 5 * time.Second
	)

	var wg sync.WaitGroup
	var readCount int64
	var writeCount int64
	var readErrors int64
	var writeErrors int64

	stopCh := make(chan struct{})

	// Start readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()

			for {
				select {
				case <-stopCh:
					return
				default:
					url := fmt.Sprintf("blocked%d.com", readerID%10000)
					result := store.IsBlocked(url)

					if result != nil {
						atomic.AddInt64(&readCount, 1)
					} else {
						atomic.AddInt64(&readErrors, 1)
					}
				}
			}
		}(i)
	}

	// Start writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()

			for j := 0; j < numOps; j++ {
				select {
				case <-stopCh:
					return
				default:
					registry := createSmallConcurrentRegistry(1000, writerID*1000+j)
					err := store.Update(registry)

					if err == nil {
						atomic.AddInt64(&writeCount, 1)
					} else {
						atomic.AddInt64(&writeErrors, 1)
					}

					time.Sleep(5 * time.Millisecond)
				}
			}
		}(i)
	}

	// Stop after duration
	time.Sleep(duration)
	close(stopCh)
	wg.Wait()

	t.Logf("Reads: %d, Read errors: %d", atomic.LoadInt64(&readCount), atomic.LoadInt64(&readErrors))
	t.Logf("Writes: %d, Write errors: %d", atomic.LoadInt64(&writeCount), atomic.LoadInt64(&writeErrors))

	if atomic.LoadInt64(&readErrors) > 0 {
		t.Errorf("Got %d read errors during concurrent operations", atomic.LoadInt64(&readErrors))
	}

	if atomic.LoadInt64(&writeErrors) > 0 {
		t.Errorf("Got %d write errors during concurrent operations", atomic.LoadInt64(&writeErrors))
	}
}

func TestMemoryStore_RaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	store := NewMemoryStore()

	const (
		numGoroutines = 100
		numOperations = 100
	)

	var wg sync.WaitGroup

	// Mix of reads, writes, and stats operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				switch j % 4 {
				case 0:
					// Read operation
					url := fmt.Sprintf("test%d.com", id)
					store.IsBlocked(url)

				case 1:
					// Write operation
					registry := createSmallConcurrentRegistry(10, id*numOperations+j)
					store.Update(registry)

				case 2:
					// Stats operation
					store.Stats()

				case 3:
					// Clear operation (occasionally)
					if j%50 == 0 {
						store.Clear()
					}
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify store is still functional
	finalRegistry := createSmallConcurrentRegistry(100, 99999)
	err := store.Update(finalRegistry)
	if err != nil {
		t.Errorf("Store became non-functional after race condition test: %v", err)
	}

	result := store.IsBlocked("blocked0.com")
	if result == nil {
		t.Error("Store not responding after race condition test")
	}
}

func TestMemoryStore_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	store := NewMemoryStore()

	const (
		registrySize  = 100000
		numReaders    = 200
		numOperations = 10000
		testDuration  = 10 * time.Second
	)

	// Load large registry
	registry := createLargeConcurrentRegistry(registrySize)
	err := store.Update(registry)
	if err != nil {
		t.Fatalf("Failed to load large registry: %v", err)
	}

	var wg sync.WaitGroup
	var totalOps int64
	var errors int64

	stopCh := make(chan struct{})

	// Start concurrent readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()

			for {
				select {
				case <-stopCh:
					return
				default:
					url := fmt.Sprintf("blocked%d.com", readerID%registrySize)
					result := store.IsBlocked(url)

					atomic.AddInt64(&totalOps, 1)

					if result == nil {
						atomic.AddInt64(&errors, 1)
					}
				}
			}
		}(i)
	}

	// Run for specified duration
	time.Sleep(testDuration)
	close(stopCh)
	wg.Wait()

	ops := atomic.LoadInt64(&totalOps)
	errs := atomic.LoadInt64(&errors)

	t.Logf("Completed %d operations in %v", ops, testDuration)
	t.Logf("Operations per second: %.0f", float64(ops)/testDuration.Seconds())
	t.Logf("Error rate: %.2f%%", float64(errs)/float64(ops)*100)

	if errs > 0 {
		t.Errorf("Got %d errors out of %d operations", errs, ops)
	}

	// Verify minimum throughput
	minThroughput := float64(50000) // 50k ops/sec minimum
	actualThroughput := float64(ops) / testDuration.Seconds()

	if actualThroughput < minThroughput {
		t.Errorf("Throughput too low: %.0f ops/sec < %.0f ops/sec", actualThroughput, minThroughput)
	}
}

func TestMemoryStore_MemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory usage test in short mode")
	}

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	store := NewMemoryStore()

	const registrySize = 1000000 // 1M entries
	registry := createLargeConcurrentRegistry(registrySize)

	err := store.Update(registry)
	if err != nil {
		t.Fatalf("Failed to update store: %v", err)
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	memoryUsed := m2.Alloc - m1.Alloc
	memoryUsedMB := float64(memoryUsed) / (1024 * 1024)

	t.Logf("Memory used for %d entries: %.2f MB", registrySize, memoryUsedMB)

	// Performance requirement: < 500MB for 1M entries
	maxMemoryMB := float64(500)
	if memoryUsedMB > maxMemoryMB {
		t.Errorf("Memory usage too high: %.2f MB > %.2f MB", memoryUsedMB, maxMemoryMB)
	}

	// Test that lookups still work with large dataset
	result := store.IsBlocked("blocked500000.com")
	if result == nil {
		t.Error("Store not responding with large dataset")
	}

	if !result.IsBlocked {
		t.Error("Expected blocked result for known blocked domain")
	}
}

func TestMemoryStore_UpdateConsistency(t *testing.T) {
	store := NewMemoryStore()

	const numUpdaters = 10
	const numUpdates = 100

	var wg sync.WaitGroup
	var updateCount int64

	// Concurrent updates
	for i := 0; i < numUpdaters; i++ {
		wg.Add(1)
		go func(updaterID int) {
			defer wg.Done()

			for j := 0; j < numUpdates; j++ {
				registry := createSmallConcurrentRegistry(100, updaterID*numUpdates+j)
				err := store.Update(registry)

				if err == nil {
					atomic.AddInt64(&updateCount, 1)
				}

				// Verify store is consistent after each update
				stats := store.Stats()
				if stats.TotalEntries < 0 {
					t.Errorf("Negative entry count: %d", stats.TotalEntries)
				}
			}
		}(i)
	}

	wg.Wait()

	expectedUpdates := int64(numUpdaters * numUpdates)
	actualUpdates := atomic.LoadInt64(&updateCount)

	if actualUpdates != expectedUpdates {
		t.Errorf("Expected %d updates, got %d", expectedUpdates, actualUpdates)
	}

	// Final consistency check
	stats := store.Stats()
	if stats.TotalEntries != 100 { // Last update should have 100 entries
		t.Errorf("Final entry count inconsistent: %d", stats.TotalEntries)
	}
}

func createLargeConcurrentRegistry(size int) *domain.Registry {
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

func createSmallConcurrentRegistry(size int, offset int) *domain.Registry {
	registry := domain.NewRegistry()

	for i := 0; i < size; i++ {
		entry, err := domain.NewRegistryEntry(domain.BlockingTypeDomain, fmt.Sprintf("blocked%d.com", offset+i))
		if err == nil {
			registry.AddEntry(entry)
		}
	}

	return registry
}
