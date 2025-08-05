package domain

import (
	"strings"
	"time"
)

type RegistryEntry struct {
	ID          string
	Type        BlockingType
	Domain      string
	IP          string
	URL         string
	Paths       []string
	AddedDate   time.Time
	BlockedDate time.Time
	Decision    string
	DecisionOrg string
}

func NewRegistryEntry(entryType BlockingType, value string) (*RegistryEntry, error) {
	if value == "" {
		return nil, ErrRegistryEntryInvalid
	}

	entry := &RegistryEntry{
		Type:      entryType,
		AddedDate: time.Now(),
	}

	switch entryType {
	case BlockingTypeDomain, BlockingTypeWildcard, BlockingTypeSNI:
		if !IsValidDomain(value) && !strings.HasPrefix(value, "*.") {
			return nil, ErrInvalidDomain
		}
		entry.Domain = value
	case BlockingTypeIP:
		if !IsValidIP(value) {
			return nil, ErrInvalidIP
		}
		entry.IP = value
	case BlockingTypeURLPath:
		entry.URL = value
	default:
		return nil, ErrRegistryEntryInvalid
	}

	return entry, nil
}

func (re *RegistryEntry) ToBlockingRule() (*BlockingRule, error) {
	var pattern string

	switch re.Type {
	case BlockingTypeDomain, BlockingTypeWildcard, BlockingTypeSNI:
		pattern = re.Domain
	case BlockingTypeIP:
		pattern = re.IP
	case BlockingTypeURLPath:
		pattern = re.URL
	default:
		return nil, ErrBlockingRuleInvalid
	}

	rule, err := NewBlockingRule(re.Type, pattern)
	if err != nil {
		return nil, err
	}

	rule.Paths = re.Paths
	return rule, nil
}

func (re *RegistryEntry) IsValid() bool {
	switch re.Type {
	case BlockingTypeDomain, BlockingTypeWildcard, BlockingTypeSNI:
		return re.Domain != ""
	case BlockingTypeIP:
		return re.IP != ""
	case BlockingTypeURLPath:
		return re.URL != ""
	default:
		return false
	}
}

type Registry struct {
	Entries     []*RegistryEntry
	LastUpdated time.Time
	Version     string
	Source      string
	EntryCount  int
}

func NewRegistry() *Registry {
	return &Registry{
		Entries:     make([]*RegistryEntry, 0),
		LastUpdated: time.Now(),
	}
}

func (r *Registry) AddEntry(entry *RegistryEntry) error {
	if entry == nil || !entry.IsValid() {
		return ErrRegistryEntryInvalid
	}

	r.Entries = append(r.Entries, entry)
	r.EntryCount++
	return nil
}

func (r *Registry) GetEntriesByType(blockingType BlockingType) []*RegistryEntry {
	var entries []*RegistryEntry
	for _, entry := range r.Entries {
		if entry.Type == blockingType {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (r *Registry) Size() int {
	return len(r.Entries)
}
