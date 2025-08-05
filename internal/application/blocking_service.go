package application

import (
	"context"
	"time"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

type BlockingService struct {
	normalizer URLNormalizer
	store      RegistryStore
}

func NewBlockingService(normalizer URLNormalizer, store RegistryStore) *BlockingService {
	return &BlockingService{
		normalizer: normalizer,
		store:      store,
	}
}

func (bs *BlockingService) CheckURL(ctx context.Context, rawURL string) (*domain.BlockingResult, error) {
	if rawURL == "" {
		return nil, domain.ErrEmptyURL
	}

	url, err := domain.NewURL(rawURL)
	if err != nil {
		return nil, err
	}

	if err := bs.normalizer.NormalizeURL(url); err != nil {
		return nil, err
	}

	if !url.IsValid() {
		return nil, domain.ErrInvalidURL
	}

	result := bs.store.IsBlocked(url.Normalized())

	if result == nil {
		return domain.NewBlockingResult(false, url.Normalized(), nil), nil
	}

	return result, nil
}

func (bs *BlockingService) GetStats(ctx context.Context) (*BlockingStats, error) {
	stats := bs.store.Stats()

	return &BlockingStats{
		TotalEntries:    stats.TotalEntries,
		DomainEntries:   stats.DomainEntries,
		WildcardEntries: stats.WildcardEntries,
		IPEntries:       stats.IPEntries,
		URLPatterns:     stats.URLPatterns,
		LastUpdate:      stats.LastUpdate.Format(time.RFC3339),
		Version:         stats.Version,
	}, nil
}

func (bs *BlockingService) UpdateRegistry(ctx context.Context, registry *domain.Registry) error {
	if registry == nil {
		return domain.ErrRegistryEntryInvalid
	}

	return bs.store.Update(registry)
}

func (bs *BlockingService) ClearRegistry(ctx context.Context) {
	bs.store.Clear()
}
