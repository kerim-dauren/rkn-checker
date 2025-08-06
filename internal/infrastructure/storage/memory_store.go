package storage

import (
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

type MemoryStore struct {
	mu sync.RWMutex

	domains     map[string]*domain.BlockingRule
	wildcards   *RadixTree
	ips         map[string]*domain.BlockingRule
	urlPatterns map[string][]*domain.BlockingRule

	bloom *BloomFilter

	lastUpdate time.Time
	entryCount int64
	version    string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		domains:     make(map[string]*domain.BlockingRule),
		wildcards:   NewRadixTree(),
		ips:         make(map[string]*domain.BlockingRule),
		urlPatterns: make(map[string][]*domain.BlockingRule),
		bloom:       NewBloomFilter(1000000, 0.01),
		lastUpdate:  time.Now(),
	}
}

func (ms *MemoryStore) IsBlocked(normalizedURL string) *domain.BlockingResult {
	if normalizedURL == "" {
		return domain.NewBlockingResult(false, normalizedURL, nil)
	}

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if !ms.bloom.Contains(normalizedURL) {
		return domain.NewBlockingResult(false, normalizedURL, nil)
	}

	if rule, exists := ms.domains[normalizedURL]; exists {
		return domain.NewBlockingResult(true, normalizedURL, rule)
	}

	if rule, exists := ms.ips[normalizedURL]; exists {
		return domain.NewBlockingResult(true, normalizedURL, rule)
	}

	if value, exists := ms.wildcards.MatchesWildcard(normalizedURL); exists {
		if rule, ok := value.(*domain.BlockingRule); ok {
			return domain.NewBlockingResult(true, normalizedURL, rule)
		}
	}

	if patterns, exists := ms.urlPatterns[normalizedURL]; exists {
		for _, rule := range patterns {
			if rule.Matches(&domain.URL{}) {
				return domain.NewBlockingResult(true, normalizedURL, rule)
			}
		}
	}

	return domain.NewBlockingResult(false, normalizedURL, nil)
}

func (ms *MemoryStore) Update(registry *domain.Registry) error {
	if registry == nil {
		return domain.ErrRegistryEntryInvalid
	}

	newDomains := make(map[string]*domain.BlockingRule)
	newWildcards := NewRadixTree()
	newIPs := make(map[string]*domain.BlockingRule)
	newURLPatterns := make(map[string][]*domain.BlockingRule)
	newBloom := NewBloomFilter(uint64(len(registry.Entries)), 0.01)

	for _, entry := range registry.Entries {
		rule, err := entry.ToBlockingRule()
		if err != nil {
			continue
		}

		switch entry.Type {
		case domain.BlockingTypeDomain:
			newDomains[entry.Domain] = rule
			newBloom.Add(entry.Domain)

		case domain.BlockingTypeWildcard:
			pattern := entry.Domain
			if strings.HasPrefix(pattern, "*.") {
				pattern = strings.TrimPrefix(pattern, "*.")
			}
			newWildcards.Insert(pattern, rule)
			newBloom.Add(pattern)

		case domain.BlockingTypeIP:
			newIPs[entry.IP] = rule
			newBloom.Add(entry.IP)

		case domain.BlockingTypeURLPath:
			domain := extractDomainFromURL(entry.URL)
			if domain != "" {
				newURLPatterns[domain] = append(newURLPatterns[domain], rule)
				newBloom.Add(domain)
			}

		case domain.BlockingTypeSNI:
			newDomains[entry.Domain] = rule
			newBloom.Add(entry.Domain)
		}
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.domains = newDomains
	ms.wildcards = newWildcards
	ms.ips = newIPs
	ms.urlPatterns = newURLPatterns
	ms.bloom = newBloom
	ms.lastUpdate = time.Now()
	ms.version = registry.Version

	atomic.StoreInt64(&ms.entryCount, int64(len(registry.Entries)))

	return nil
}

func (ms *MemoryStore) Stats() StoreStats {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	return StoreStats{
		TotalEntries:    atomic.LoadInt64(&ms.entryCount),
		DomainEntries:   int64(len(ms.domains)),
		WildcardEntries: int64(ms.wildcards.Size()),
		IPEntries:       int64(len(ms.ips)),
		URLPatterns:     int64(len(ms.urlPatterns)),
		LastUpdate:      ms.lastUpdate,
		Version:         ms.version,
		BloomFilterSize: ms.bloom.Size(),
	}
}

func (ms *MemoryStore) Clear() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.domains = make(map[string]*domain.BlockingRule)
	ms.wildcards.Clear()
	ms.ips = make(map[string]*domain.BlockingRule)
	ms.urlPatterns = make(map[string][]*domain.BlockingRule)
	ms.bloom.Clear()

	atomic.StoreInt64(&ms.entryCount, 0)
	ms.lastUpdate = time.Now()
}

type StoreStats struct {
	TotalEntries    int64
	DomainEntries   int64
	WildcardEntries int64
	IPEntries       int64
	URLPatterns     int64
	LastUpdate      time.Time
	Version         string
	BloomFilterSize uint64
}

func extractDomainFromURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	return parsedURL.Host
}
