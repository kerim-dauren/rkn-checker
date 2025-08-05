package application

import (
	"context"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
	"github.com/kerim-dauren/rkn-checker/internal/infrastructure/storage"
)

type URLNormalizer interface {
	Normalize(rawURL string) (string, error)
	NormalizeURL(url *domain.URL) error
}

type RegistryStore interface {
	IsBlocked(normalizedURL string) *domain.BlockingResult
	Update(registry *domain.Registry) error
	Stats() storage.StoreStats
	Clear()
}

type BlockingChecker interface {
	CheckURL(ctx context.Context, rawURL string) (*domain.BlockingResult, error)
	GetStats(ctx context.Context) (*BlockingStats, error)
}

type BlockingStats struct {
	TotalEntries    int64  `json:"total_entries"`
	DomainEntries   int64  `json:"domain_entries"`
	WildcardEntries int64  `json:"wildcard_entries"`
	IPEntries       int64  `json:"ip_entries"`
	URLPatterns     int64  `json:"url_patterns"`
	LastUpdate      string `json:"last_update"`
	Version         string `json:"version"`
}
