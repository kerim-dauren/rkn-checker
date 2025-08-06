package application

import (
	"context"
	"fmt"
	"testing"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
	"github.com/kerim-dauren/rkn-checker/internal/domain/services"
	"github.com/kerim-dauren/rkn-checker/internal/infrastructure/storage"
)

func BenchmarkBlockingService_CheckURL_Small(b *testing.B) {
	service := createBenchmarkService(1000)
	ctx := context.Background()
	urls := generateBenchmarkURLs(100)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			url := urls[i%len(urls)]
			_, _ = service.CheckURL(ctx, url)
			i++
		}
	})
}

func BenchmarkBlockingService_CheckURL_Medium(b *testing.B) {
	service := createBenchmarkService(100000)
	ctx := context.Background()
	urls := generateBenchmarkURLs(1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			url := urls[i%len(urls)]
			_, _ = service.CheckURL(ctx, url)
			i++
		}
	})
}

func BenchmarkBlockingService_CheckURL_Large(b *testing.B) {
	service := createBenchmarkService(1000000)
	ctx := context.Background()
	urls := generateBenchmarkURLs(10000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			url := urls[i%len(urls)]
			_, _ = service.CheckURL(ctx, url)
			i++
		}
	})
}

func TestPerformanceRequirements(t *testing.T) {
	service := createBenchmarkService(1000000)
	ctx := context.Background()

	const (
		numRequests   = 10000
		maxLatencyMs  = 1.0   // < 1ms P99 requirement
		minThroughput = 10000 // > 10k req/s requirement
	)

	urls := generateBenchmarkURLs(1000)

	result := testing.Benchmark(func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				url := urls[i%len(urls)]
				_, _ = service.CheckURL(ctx, url)
				i++
			}
		})
	})

	avgLatencyNs := result.NsPerOp()
	avgLatencyMs := float64(avgLatencyNs) / 1e6
	throughput := float64(1e9) / float64(avgLatencyNs)

	t.Logf("Average latency: %.3f ms", avgLatencyMs)
	t.Logf("Estimated throughput: %.0f req/s", throughput)

	if avgLatencyMs > maxLatencyMs {
		t.Errorf("Latency requirement failed: %.3f ms > %.3f ms", avgLatencyMs, maxLatencyMs)
	}

	if throughput < minThroughput {
		t.Errorf("Throughput requirement failed: %.0f req/s < %.0f req/s", throughput, float64(minThroughput))
	}
}

func createBenchmarkService(registrySize int) *BlockingService {
	normalizer := services.NewURLNormalizer()
	store := storage.NewMemoryStore()
	registry := createBenchmarkRegistry(registrySize)

	store.Update(registry)

	return NewBlockingService(normalizer, store)
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

func generateBenchmarkURLs(count int) []string {
	urls := make([]string, count)

	for i := 0; i < count; i++ {
		switch i % 5 {
		case 0:
			urls[i] = fmt.Sprintf("https://blocked%d.com", i%1000)
		case 1:
			urls[i] = fmt.Sprintf("https://sub.wildcard%d.com", i%1000)
		case 2:
			urls[i] = fmt.Sprintf("https://192.%d.%d.%d", (i/65536)%256, (i/256)%256, i%256)
		case 3:
			urls[i] = fmt.Sprintf("https://safe%d.com", i)
		case 4:
			urls[i] = fmt.Sprintf("https://UPPERCASE%d.COM:8080/path", i)
		}
	}

	return urls
}
