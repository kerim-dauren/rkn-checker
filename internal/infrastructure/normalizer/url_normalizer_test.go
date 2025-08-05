package normalizer

import (
	"testing"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

func TestURLNormalizer_Normalize(t *testing.T) {
	normalizer := NewURLNormalizer()

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		// Protocol removal
		{"https protocol", "https://example.com", "example.com", false},
		{"http protocol", "http://example.com", "example.com", false},
		{"ftp protocol", "ftp://example.com", "example.com", false},
		{"no protocol", "example.com", "example.com", false},

		// Port removal
		{"standard https port", "https://example.com:443", "example.com", false},
		{"standard http port", "http://example.com:80", "example.com", false},
		{"custom port", "https://example.com:8080", "example.com", false},

		// Case normalization
		{"uppercase domain", "HTTPS://EXAMPLE.COM", "example.com", false},
		{"mixed case", "https://ExAmPlE.cOm", "example.com", false},

		// WWW handling
		{"www prefix", "https://www.example.com", "example.com", false},
		{"www subdomain preserved", "https://www.sub.example.com", "sub.example.com", false},

		// Subdomain preservation
		{"api subdomain", "https://api.example.com", "api.example.com", false},
		{"deep subdomain", "https://deep.sub.example.com", "deep.sub.example.com", false},

		// Path/query/fragment removal
		{"with path", "https://example.com/path", "example.com", false},
		{"with query", "https://example.com?query=1", "example.com", false},
		{"with fragment", "https://example.com#fragment", "example.com", false},
		{"with all", "https://example.com/path?query=1#fragment", "example.com", false},

		// IP addresses
		{"IPv4", "http://192.168.1.1", "192.168.1.1", false},
		{"IPv6", "https://[2001:db8::1]", "2001:db8::1", false},
		{"IPv6 full", "https://[2001:0db8:85a3:0000:0000:8a2e:0370:7334]", "2001:db8:85a3::8a2e:370:7334", false},

		// Error cases
		{"empty URL", "", "", true},
		{"invalid URL", "not-a-url", "", true},
		{"protocol only", "https://", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := normalizer.Normalize(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Normalize() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Normalize() unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("Normalize() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestURLNormalizer_NormalizeURL(t *testing.T) {
	normalizer := NewURLNormalizer()

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{"valid URL", "https://example.com", "example.com", false},
		{"complex URL", "HTTPS://WWW.EXAMPLE.COM:8080/path?q=1#fragment", "example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := domain.NewURL(tt.input)
			if err != nil {
				t.Fatalf("NewURL() failed: %v", err)
			}

			err = normalizer.NormalizeURL(url)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NormalizeURL() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NormalizeURL() unexpected error: %v", err)
				return
			}

			if url.Normalized() != tt.expected {
				t.Errorf("NormalizeURL() = %v, want %v", url.Normalized(), tt.expected)
			}

			if !url.IsValid() {
				t.Errorf("NormalizeURL() resulted in invalid URL")
			}
		})
	}
}

func TestURLNormalizer_IDN_Support(t *testing.T) {
	normalizer := NewURLNormalizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Russian domain", "https://тест.рф", "xn--e1aybc.xn--p1ai"},
		{"German domain", "https://münchen.de", "xn--mnchen-3ya.de"},
		{"Chinese domain", "https://测试.中国", "xn--0zwm56d.xn--fiqs8s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := normalizer.Normalize(tt.input)
			if err != nil {
				t.Errorf("Normalize() unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("Normalize() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func BenchmarkURLNormalizer_Normalize(b *testing.B) {
	normalizer := NewURLNormalizer()
	urls := []string{
		"https://example.com",
		"HTTPS://WWW.EXAMPLE.COM:8080/path?query=1#fragment",
		"http://тест.рф",
		"https://192.168.1.1",
		"https://[2001:db8::1]",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		url := urls[i%len(urls)]
		_, _ = normalizer.Normalize(url)
	}
}
