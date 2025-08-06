package registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OfficialSource implements Source interface for official RKN API
type OfficialSource struct {
	client     *http.Client
	config     SourceConfig
	lastHealth time.Time
	healthy    bool
}

// NewOfficialSource creates a new official RKN API source
func NewOfficialSource(config SourceConfig) *OfficialSource {
	return &OfficialSource{
		client: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:       5,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
			},
		},
		config:  config,
		healthy: true,
	}
}

// Name returns the source name
func (o *OfficialSource) Name() string {
	return "Official RKN API"
}

// Fetch retrieves registry data from official RKN API
// Note: This may be blocked when accessed from Germany
func (o *OfficialSource) Fetch(ctx context.Context) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt < o.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter
			backoff := time.Duration(attempt*attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		data, err := o.fetchOnce(ctx)
		if err == nil {
			o.healthy = true
			o.lastHealth = time.Now()
			return data, nil
		}

		lastErr = err
	}

	o.healthy = false
	return nil, NewSourceError(o.Name(), "fetch", lastErr)
}

// fetchOnce performs a single fetch attempt
func (o *OfficialSource) fetchOnce(ctx context.Context) ([]byte, error) {
	// The official RKN API typically requires authentication and special procedures
	// This is a simplified implementation that would need to be adapted
	// based on the actual API specification

	req, err := http.NewRequestWithContext(ctx, "GET", o.config.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set headers for official API
	req.Header.Set("User-Agent", o.config.UserAgent)
	req.Header.Set("Accept", "application/xml, application/zip, text/csv")
	req.Header.Set("Cache-Control", "no-cache")

	// Note: Real implementation would need to handle authentication
	// and possibly POST requests with specific parameters

	resp, err := o.client.Do(req)
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
func (o *OfficialSource) IsHealthy(ctx context.Context) bool {
	// Return cached health status if checked recently
	if time.Since(o.lastHealth) < 5*time.Minute {
		return o.healthy
	}

	// Perform health check with HEAD request
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "HEAD", o.config.URL, nil)
	if err != nil {
		o.healthy = false
		return false
	}

	req.Header.Set("User-Agent", o.config.UserAgent)

	resp, err := o.client.Do(req)
	if err != nil {
		o.healthy = false
		return false
	}
	defer resp.Body.Close()

	// Official API might return different status codes
	// Accept both 200 and 405 (Method Not Allowed) as healthy
	o.healthy = resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusMethodNotAllowed
	o.lastHealth = time.Now()

	return o.healthy
}
