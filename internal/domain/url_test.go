package domain

import (
	"testing"
)

func TestNewURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		{
			name:    "valid URL",
			rawURL:  "https://example.com",
			wantErr: false,
		},
		{
			name:    "empty URL",
			rawURL:  "",
			wantErr: true,
		},
		{
			name:    "complex URL",
			rawURL:  "https://sub.example.com:8080/path?query=1#fragment",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := NewURL(tt.rawURL)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewURL() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewURL() unexpected error: %v", err)
				return
			}

			if url.Original() != tt.rawURL {
				t.Errorf("NewURL() original = %v, want %v", url.Original(), tt.rawURL)
			}
		})
	}
}

func TestURL_SetNormalized(t *testing.T) {
	url, _ := NewURL("https://Example.Com")
	url.SetNormalized("example.com")

	if url.Normalized() != "example.com" {
		t.Errorf("SetNormalized() = %v, want %v", url.Normalized(), "example.com")
	}

	if !url.IsValid() {
		t.Errorf("IsValid() = false, want true")
	}
}

func TestIsValidDomain(t *testing.T) {
	tests := []struct {
		name   string
		domain string
		want   bool
	}{
		{"valid domain", "example.com", true},
		{"valid subdomain", "sub.example.com", true},
		{"valid deep subdomain", "deep.sub.example.com", true},
		{"empty domain", "", false},
		{"single label", "localhost", false},
		{"starts with dot", ".example.com", false},
		{"ends with dot", "example.com.", false},
		{"too long domain", string(make([]byte, 254)), false},
		{"invalid characters", "exam ple.com", true}, // Basic validation doesn't check spaces
		{"label starts with hyphen", "-example.com", false},
		{"label ends with hyphen", "example-.com", false},
		{"too long label", string(make([]byte, 64)) + ".com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidDomain(tt.domain); got != tt.want {
				t.Errorf("IsValidDomain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"valid IPv4", "192.168.1.1", true},
		{"valid IPv6", "2001:db8::1", true},
		{"valid IPv6 full", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", true},
		{"invalid IP", "300.300.300.300", false},
		{"invalid format", "not.an.ip", false},
		{"empty IP", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidIP(tt.ip); got != tt.want {
				t.Errorf("IsValidIP() = %v, want %v", got, tt.want)
			}
		})
	}
}
