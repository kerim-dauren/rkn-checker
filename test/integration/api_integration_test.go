package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/kerim-dauren/rkn-checker/internal/application"
	grpcServer "github.com/kerim-dauren/rkn-checker/internal/delivery/grpc"
	"github.com/kerim-dauren/rkn-checker/internal/delivery/grpc/proto"
	restServer "github.com/kerim-dauren/rkn-checker/internal/delivery/rest"
	"github.com/kerim-dauren/rkn-checker/internal/domain"
	"github.com/kerim-dauren/rkn-checker/internal/domain/services"
	"github.com/kerim-dauren/rkn-checker/internal/infrastructure/storage"
)

func setupTestService() application.BlockingChecker {
	normalizer := services.NewURLNormalizer()
	store := storage.NewMemoryStore()
	service := application.NewBlockingService(normalizer, store)

	registry := domain.NewRegistry()
	
	domains := []string{"blocked.com", "example-blocked.org"}
	for _, d := range domains {
		entry, _ := domain.NewRegistryEntry(domain.BlockingTypeDomain, d)
		registry.AddEntry(entry)
	}

	wildcards := []string{"*.wildcard.com", "*.ads.example.com"}
	for _, w := range wildcards {
		entry, _ := domain.NewRegistryEntry(domain.BlockingTypeWildcard, w)
		registry.AddEntry(entry)
	}

	ips := []string{"192.168.1.100"}
	for _, ip := range ips {
		entry, _ := domain.NewRegistryEntry(domain.BlockingTypeIP, ip)
		registry.AddEntry(entry)
	}

	service.UpdateRegistry(context.Background(), registry)
	return service
}

func TestRESTAPIIntegration(t *testing.T) {
	service := setupTestService()
	server := restServer.NewServer(service, 8081)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		server.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("Health Check", func(t *testing.T) {
		resp, err := client.Get("http://localhost:8081/health")
		if err != nil {
			t.Fatalf("Failed to call health endpoint: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var health restServer.HealthResponse
		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
			t.Fatalf("Failed to decode health response: %v", err)
		}

		if health.Status != "healthy" {
			t.Errorf("Expected status 'healthy', got '%s'", health.Status)
		}
	})

	t.Run("Get Stats", func(t *testing.T) {
		resp, err := client.Get("http://localhost:8081/api/v1/stats")
		if err != nil {
			t.Fatalf("Failed to call stats endpoint: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var stats restServer.StatsResponse
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			t.Fatalf("Failed to decode stats response: %v", err)
		}

		if stats.TotalEntries != 5 {
			t.Errorf("Expected 5 total entries, got %d", stats.TotalEntries)
		}
	})

	t.Run("Check URL - Allowed", func(t *testing.T) {
		reqBody := restServer.CheckURLRequest{URL: "https://allowed.com"}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post("http://localhost:8081/api/v1/check", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to call check endpoint: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var result restServer.CheckURLResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode check response: %v", err)
		}

		if result.Blocked {
			t.Errorf("Expected URL to be allowed, but it was blocked")
		}

		if result.NormalizedURL != "allowed.com" {
			t.Errorf("Expected normalized URL 'allowed.com', got '%s'", result.NormalizedURL)
		}
	})

	t.Run("Check URL - Blocked Domain", func(t *testing.T) {
		reqBody := restServer.CheckURLRequest{URL: "https://blocked.com/path"}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post("http://localhost:8081/api/v1/check", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to call check endpoint: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var result restServer.CheckURLResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode check response: %v", err)
		}

		if !result.Blocked {
			t.Errorf("Expected URL to be blocked, but it was allowed")
		}

		if result.Reason != "domain" {
			t.Errorf("Expected reason 'domain', got '%s'", result.Reason)
		}
	})

	t.Run("Check URL - Blocked Wildcard", func(t *testing.T) {
		reqBody := restServer.CheckURLRequest{URL: "https://sub.wildcard.com"}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post("http://localhost:8081/api/v1/check", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to call check endpoint: %v", err)
		}
		defer resp.Body.Close()

		var result restServer.CheckURLResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode check response: %v", err)
		}

		if !result.Blocked {
			t.Errorf("Expected URL to be blocked by wildcard, but it was allowed")
		}
	})

	cancel()
}

func TestGRPCAPIIntegration(t *testing.T) {
	service := setupTestService()
	server := grpcServer.NewServer(service, 9091)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		server.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	conn, err := grpc.Dial("localhost:9091", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	client := proto.NewBlockingServiceClient(conn)

	t.Run("Health Check", func(t *testing.T) {
		resp, err := client.HealthCheck(context.Background(), &proto.HealthCheckRequest{})
		if err != nil {
			t.Fatalf("Failed to call health check: %v", err)
		}

		if resp.Status != proto.HealthCheckResponse_SERVING {
			t.Errorf("Expected status SERVING, got %v", resp.Status)
		}
	})

	t.Run("Get Stats", func(t *testing.T) {
		resp, err := client.GetStats(context.Background(), &proto.GetStatsRequest{})
		if err != nil {
			t.Fatalf("Failed to call get stats: %v", err)
		}

		if resp.TotalEntries != 5 {
			t.Errorf("Expected 5 total entries, got %d", resp.TotalEntries)
		}
	})

	t.Run("Check URL - Allowed", func(t *testing.T) {
		resp, err := client.CheckURL(context.Background(), &proto.CheckURLRequest{
			Url: "https://allowed.com",
		})
		if err != nil {
			t.Fatalf("Failed to check URL: %v", err)
		}

		if resp.Blocked {
			t.Errorf("Expected URL to be allowed, but it was blocked")
		}

		if resp.NormalizedUrl != "allowed.com" {
			t.Errorf("Expected normalized URL 'allowed.com', got '%s'", resp.NormalizedUrl)
		}
	})

	t.Run("Check URL - Blocked Domain", func(t *testing.T) {
		resp, err := client.CheckURL(context.Background(), &proto.CheckURLRequest{
			Url: "https://blocked.com/path",
		})
		if err != nil {
			t.Fatalf("Failed to check URL: %v", err)
		}

		if !resp.Blocked {
			t.Errorf("Expected URL to be blocked, but it was allowed")
		}

		if resp.Reason != "domain" {
			t.Errorf("Expected reason 'domain', got '%s'", resp.Reason)
		}
	})

	t.Run("Check URL - Invalid URL", func(t *testing.T) {
		_, err := client.CheckURL(context.Background(), &proto.CheckURLRequest{
			Url: "",
		})
		if err == nil {
			t.Errorf("Expected error for empty URL, but got none")
		}
	})

	cancel()
}