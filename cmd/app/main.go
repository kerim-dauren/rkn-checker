package main

import (
	"context"
	"fmt"
	"log"

	"github.com/kerim-dauren/rkn-checker/internal/application"
	"github.com/kerim-dauren/rkn-checker/internal/domain"
	"github.com/kerim-dauren/rkn-checker/internal/infrastructure/normalizer"
	"github.com/kerim-dauren/rkn-checker/internal/infrastructure/storage"
)

func main() {
	fmt.Println("=== Phase 1 Demo: Roskomnadzor URL Blocking Service ===")
	fmt.Println()

	// Initialize components
	fmt.Println("Initializing components...")
	normalizer := normalizer.NewURLNormalizer()
	store := storage.NewMemoryStore()
	service := application.NewBlockingService(normalizer, store)

	// Create sample registry
	fmt.Println("Creating sample registry...")
	registry := createSampleRegistry()

	err := service.UpdateRegistry(context.Background(), registry)
	if err != nil {
		log.Fatalf("Failed to update registry: %v", err)
	}

	// Display registry statistics
	stats, _ := service.GetStats(context.Background())
	fmt.Printf("Registry loaded: %d total entries\n", stats.TotalEntries)
	fmt.Printf("- Domain entries: %d\n", stats.DomainEntries)
	fmt.Printf("- Wildcard entries: %d\n", stats.WildcardEntries)
	fmt.Printf("- IP entries: %d\n", stats.IPEntries)
	fmt.Println()

	// Test URLs
	testURLs := []string{
		"https://blocked.com",
		"http://safe.com",
		"https://sub.wildcard.com",
		"HTTPS://WWW.BLOCKED.COM:8080/path?query=1",
		"http://192.168.1.100",
		"https://тест.рф", // IDN domain
		"not-a-valid-url",
	}

	fmt.Println("Testing URL blocking:")
	fmt.Println("====================================")

	for _, testURL := range testURLs {
		result, err := service.CheckURL(context.Background(), testURL)

		if err != nil {
			fmt.Printf("❌ %s -> ERROR: %v\n", testURL, err)
			continue
		}

		status := "✅ ALLOWED"
		reason := ""

		if result.IsBlocked {
			status = "🚫 BLOCKED"
			reason = fmt.Sprintf(" (%s)", result.Reason.String())
		}

		fmt.Printf("%s %s -> %s%s\n", status, testURL, result.NormalizedURL, reason)
	}
}

func createSampleRegistry() *domain.Registry {
	registry := domain.NewRegistry()

	// Sample blocked domains
	domains := []string{"blocked.com", "example-blocked.org", "тест.рф"}
	for _, d := range domains {
		entry, _ := domain.NewRegistryEntry(domain.BlockingTypeDomain, d)
		registry.AddEntry(entry)
	}

	// Sample wildcard rules
	wildcards := []string{"*.wildcard.com", "*.ads.example.com"}
	for _, w := range wildcards {
		entry, _ := domain.NewRegistryEntry(domain.BlockingTypeWildcard, w)
		registry.AddEntry(entry)
	}

	// Sample IP blocks
	ips := []string{"192.168.1.100", "10.0.0.1"}
	for _, ip := range ips {
		entry, _ := domain.NewRegistryEntry(domain.BlockingTypeIP, ip)
		registry.AddEntry(entry)
	}

	return registry
}
