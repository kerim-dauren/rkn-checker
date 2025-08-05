package domain

import (
	"testing"
)

func TestNewBlockingRule(t *testing.T) {
	tests := []struct {
		name     string
		ruleType BlockingType
		pattern  string
		wantErr  bool
	}{
		{"valid domain rule", BlockingTypeDomain, "example.com", false},
		{"valid wildcard rule", BlockingTypeWildcard, "*.example.com", false},
		{"valid IP rule", BlockingTypeIP, "192.168.1.1", false},
		{"valid URL path rule", BlockingTypeURLPath, "example.com/blocked", false},
		{"empty pattern", BlockingTypeDomain, "", true},
		{"invalid domain", BlockingTypeDomain, "invalid..domain", true},
		{"invalid IP", BlockingTypeIP, "300.300.300.300", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := NewBlockingRule(tt.ruleType, tt.pattern)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewBlockingRule() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewBlockingRule() unexpected error: %v", err)
				return
			}

			if rule.Type != tt.ruleType {
				t.Errorf("NewBlockingRule() type = %v, want %v", rule.Type, tt.ruleType)
			}

			if rule.Pattern != tt.pattern {
				t.Errorf("NewBlockingRule() pattern = %v, want %v", rule.Pattern, tt.pattern)
			}
		})
	}
}

func TestBlockingRule_Matches(t *testing.T) {
	tests := []struct {
		name string
		rule *BlockingRule
		url  string
		want bool
	}{
		{
			name: "domain exact match",
			rule: &BlockingRule{Type: BlockingTypeDomain, Pattern: "example.com"},
			url:  "example.com",
			want: true,
		},
		{
			name: "domain no match",
			rule: &BlockingRule{Type: BlockingTypeDomain, Pattern: "example.com"},
			url:  "other.com",
			want: false,
		},
		{
			name: "wildcard match subdomain",
			rule: &BlockingRule{Type: BlockingTypeWildcard, Pattern: "*.example.com"},
			url:  "sub.example.com",
			want: true,
		},
		{
			name: "wildcard match exact domain",
			rule: &BlockingRule{Type: BlockingTypeWildcard, Pattern: "*.example.com"},
			url:  "example.com",
			want: true,
		},
		{
			name: "wildcard no match",
			rule: &BlockingRule{Type: BlockingTypeWildcard, Pattern: "*.example.com"},
			url:  "other.com",
			want: false,
		},
		{
			name: "IP exact match",
			rule: &BlockingRule{Type: BlockingTypeIP, Pattern: "192.168.1.1"},
			url:  "192.168.1.1",
			want: true,
		},
		{
			name: "URL path match",
			rule: &BlockingRule{Type: BlockingTypeURLPath, Pattern: "example.com/blocked"},
			url:  "example.com/blocked/page",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, _ := NewURL("http://" + tt.url)
			url.SetNormalized(tt.url)

			if got := tt.rule.Matches(url); got != tt.want {
				t.Errorf("BlockingRule.Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlockingType_String(t *testing.T) {
	tests := []struct {
		blockingType BlockingType
		want         string
	}{
		{BlockingTypeDomain, "domain"},
		{BlockingTypeWildcard, "wildcard"},
		{BlockingTypeIP, "ip"},
		{BlockingTypeURLPath, "url_path"},
		{BlockingTypeSNI, "sni"},
		{BlockingTypeUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.blockingType.String(); got != tt.want {
				t.Errorf("BlockingType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewBlockingResult(t *testing.T) {
	rule, _ := NewBlockingRule(BlockingTypeDomain, "example.com")
	result := NewBlockingResult(true, "example.com", rule)

	if !result.IsBlocked {
		t.Errorf("NewBlockingResult() IsBlocked = false, want true")
	}

	if result.NormalizedURL != "example.com" {
		t.Errorf("NewBlockingResult() NormalizedURL = %v, want %v", result.NormalizedURL, "example.com")
	}

	if result.Rule != rule {
		t.Errorf("NewBlockingResult() Rule = %v, want %v", result.Rule, rule)
	}

	if result.Reason != BlockingTypeDomain {
		t.Errorf("NewBlockingResult() Reason = %v, want %v", result.Reason, BlockingTypeDomain)
	}
}
