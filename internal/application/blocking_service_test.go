package application

import (
	"context"
	"testing"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
	"github.com/kerim-dauren/rkn-checker/internal/infrastructure/normalizer"
	"github.com/kerim-dauren/rkn-checker/internal/infrastructure/storage"
)

func TestNewBlockingService(t *testing.T) {
	normalizer := normalizer.NewURLNormalizer()
	store := storage.NewMemoryStore()

	service := NewBlockingService(normalizer, store)

	if service == nil {
		t.Fatal("NewBlockingService() returned nil")
	}
}

func TestBlockingService_CheckURL(t *testing.T) {
	service := createTestBlockingService()
	ctx := context.Background()

	tests := []struct {
		name        string
		rawURL      string
		wantBlocked bool
		wantErr     bool
	}{
		{
			name:        "blocked domain",
			rawURL:      "https://blocked.com",
			wantBlocked: true,
			wantErr:     false,
		},
		{
			name:        "wildcard match",
			rawURL:      "https://sub.wildcard.com",
			wantBlocked: true,
			wantErr:     false,
		},
		{
			name:        "not blocked",
			rawURL:      "https://safe.com",
			wantBlocked: false,
			wantErr:     false,
		},
		{
			name:        "empty URL",
			rawURL:      "",
			wantBlocked: false,
			wantErr:     true,
		},
		{
			name:        "invalid URL",
			rawURL:      "not-a-url",
			wantBlocked: false,
			wantErr:     true, // normalizer will fail on invalid domain
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.CheckURL(ctx, tt.rawURL)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CheckURL() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("CheckURL() unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Fatal("CheckURL() returned nil result")
			}

			if result.IsBlocked != tt.wantBlocked {
				t.Errorf("CheckURL() IsBlocked = %v, want %v", result.IsBlocked, tt.wantBlocked)
			}
		})
	}
}

func TestBlockingService_GetStats(t *testing.T) {
	service := createTestBlockingService()
	ctx := context.Background()

	stats, err := service.GetStats(ctx)
	if err != nil {
		t.Errorf("GetStats() unexpected error: %v", err)
		return
	}

	if stats == nil {
		t.Fatal("GetStats() returned nil")
	}

	if stats.TotalEntries == 0 {
		t.Error("GetStats() TotalEntries should be > 0")
	}
}

func TestBlockingService_UpdateRegistry(t *testing.T) {
	service := createTestBlockingService()
	ctx := context.Background()

	newRegistry := domain.NewRegistry()
	newEntry, _ := domain.NewRegistryEntry(domain.BlockingTypeDomain, "newblocked.com")
	newRegistry.AddEntry(newEntry)

	err := service.UpdateRegistry(ctx, newRegistry)
	if err != nil {
		t.Errorf("UpdateRegistry() unexpected error: %v", err)
		return
	}

	// Test that the new entry is blocked
	result, err := service.CheckURL(ctx, "https://newblocked.com")
	if err != nil {
		t.Errorf("CheckURL() after update failed: %v", err)
		return
	}

	if !result.IsBlocked {
		t.Error("CheckURL() should block newly added domain")
	}
}

func TestBlockingService_ClearRegistry(t *testing.T) {
	service := createTestBlockingService()
	ctx := context.Background()

	// Verify there are entries
	stats, _ := service.GetStats(ctx)
	if stats.TotalEntries == 0 {
		t.Fatal("Test registry should have entries")
	}

	service.ClearRegistry(ctx)

	// Verify entries are cleared
	stats, _ = service.GetStats(ctx)
	if stats.TotalEntries != 0 {
		t.Errorf("ClearRegistry() TotalEntries = %v, want 0", stats.TotalEntries)
	}
}

func BenchmarkBlockingService_CheckURL(b *testing.B) {
	service := createTestBlockingService()
	ctx := context.Background()

	urls := []string{
		"https://example.com",
		"https://blocked.com",
		"https://sub.wildcard.com",
		"https://192.168.1.100",
		"https://safe.com",
	}

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

func createTestBlockingService() *BlockingService {
	normalizer := normalizer.NewURLNormalizer()
	store := storage.NewMemoryStore()

	// Create test registry
	registry := domain.NewRegistry()

	// Add test entries
	domainEntry, _ := domain.NewRegistryEntry(domain.BlockingTypeDomain, "blocked.com")
	registry.AddEntry(domainEntry)

	wildcardEntry, _ := domain.NewRegistryEntry(domain.BlockingTypeWildcard, "*.wildcard.com")
	registry.AddEntry(wildcardEntry)

	ipEntry, _ := domain.NewRegistryEntry(domain.BlockingTypeIP, "192.168.1.100")
	registry.AddEntry(ipEntry)

	// Update store with test data
	store.Update(registry)

	return NewBlockingService(normalizer, store)
}
