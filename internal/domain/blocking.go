package domain

import (
	"strings"
	"time"
)

type BlockingType int

const (
	BlockingTypeUnknown BlockingType = iota
	BlockingTypeDomain
	BlockingTypeWildcard
	BlockingTypeIP
	BlockingTypeURLPath
	BlockingTypeSNI
)

func (bt BlockingType) String() string {
	switch bt {
	case BlockingTypeDomain:
		return "domain"
	case BlockingTypeWildcard:
		return "wildcard"
	case BlockingTypeIP:
		return "ip"
	case BlockingTypeURLPath:
		return "url_path"
	case BlockingTypeSNI:
		return "sni"
	default:
		return "unknown"
	}
}

type BlockingRule struct {
	Type     BlockingType
	Pattern  string
	Original string
	Paths    []string
}

func NewBlockingRule(ruleType BlockingType, pattern string) (*BlockingRule, error) {
	if pattern == "" {
		return nil, ErrBlockingRuleInvalid
	}

	rule := &BlockingRule{
		Type:     ruleType,
		Pattern:  pattern,
		Original: pattern,
	}

	if err := rule.validate(); err != nil {
		return nil, err
	}

	return rule, nil
}

func (br *BlockingRule) validate() error {
	switch br.Type {
	case BlockingTypeDomain:
		if !IsValidDomain(br.Pattern) {
			return ErrInvalidDomain
		}
	case BlockingTypeWildcard:
		pattern := strings.TrimPrefix(br.Pattern, "*.")
		if !IsValidDomain(pattern) {
			return ErrInvalidDomain
		}
	case BlockingTypeIP:
		if !IsValidIP(br.Pattern) {
			return ErrInvalidIP
		}
	case BlockingTypeURLPath:
		if br.Pattern == "" {
			return ErrBlockingRuleInvalid
		}
	case BlockingTypeSNI:
		if !IsValidDomain(br.Pattern) {
			return ErrInvalidDomain
		}
	default:
		return ErrBlockingRuleInvalid
	}

	return nil
}

func (br *BlockingRule) Matches(url *URL) bool {
	if url == nil || !url.IsValid() {
		return false
	}

	normalized := url.Normalized()

	switch br.Type {
	case BlockingTypeDomain:
		return normalized == br.Pattern
	case BlockingTypeWildcard:
		domain := strings.TrimPrefix(br.Pattern, "*.")
		return normalized == domain || strings.HasSuffix(normalized, "."+domain)
	case BlockingTypeIP:
		return normalized == br.Pattern
	case BlockingTypeURLPath:
		return strings.HasPrefix(normalized, br.Pattern)
	case BlockingTypeSNI:
		return normalized == br.Pattern
	default:
		return false
	}
}

type BlockingResult struct {
	IsBlocked     bool
	NormalizedURL string
	Rule          *BlockingRule
	Reason        BlockingType
	CheckedAt     time.Time
}

func NewBlockingResult(isBlocked bool, normalizedURL string, rule *BlockingRule) *BlockingResult {
	result := &BlockingResult{
		IsBlocked:     isBlocked,
		NormalizedURL: normalizedURL,
		Rule:          rule,
		CheckedAt:     time.Now(),
	}

	if rule != nil {
		result.Reason = rule.Type
	}

	return result
}
