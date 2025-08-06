package registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// GitHubSource implements Source interface for GitHub mirror
type GitHubSource struct {
	client     *http.Client
	config     SourceConfig
	lastHealth time.Time
	healthy    bool
	mu         sync.RWMutex
}

// NewGitHubSource creates a new GitHub registry source
func NewGitHubSource(config SourceConfig) *GitHubSource {
	return &GitHubSource{
		client: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
			},
		},
		config:  config,
		healthy: true,
	}
}

// Name returns the source name
func (g *GitHubSource) Name() string {
	return "GitHub Mirror"
}

// Fetch retrieves registry data from GitHub mirror
func (g *GitHubSource) Fetch(ctx context.Context) ([]byte, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	var lastErr error

	for attempt := 0; attempt < g.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(attempt*attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		data, err := g.fetchOnce(ctx)
		if err == nil {
			g.healthy = true
			g.lastHealth = time.Now()
			return data, nil
		}

		lastErr = err
	}

	g.healthy = false
	return nil, NewSourceError(g.Name(), "fetch", lastErr)
}

// fetchOnce performs a single fetch attempt
func (g *GitHubSource) fetchOnce(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", g.config.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", g.config.UserAgent)
	req.Header.Set("Accept", "text/csv, application/zip, */*")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Check content length to avoid extremely large downloads
	if resp.ContentLength > 100*1024*1024 { // 100MB limit
		return nil, fmt.Errorf("response too large: %d bytes", resp.ContentLength)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	return data, nil
}

// IsHealthy checks if the source is currently available
func (g *GitHubSource) IsHealthy(ctx context.Context) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Return cached health status if checked recently
	if time.Since(g.lastHealth) < 5*time.Minute {
		return g.healthy
	}

	// Perform health check with HEAD request
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "HEAD", g.config.URL, nil)
	if err != nil {
		g.healthy = false
		return false
	}

	req.Header.Set("User-Agent", g.config.UserAgent)

	resp, err := g.client.Do(req)
	if err != nil {
		g.healthy = false
		return false
	}
	defer resp.Body.Close()

	g.healthy = resp.StatusCode == http.StatusOK
	g.lastHealth = time.Now()

	return g.healthy
}
