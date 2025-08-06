package registry

import (
	"strings"
	"testing"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

func TestParser_detectFormat(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		data     []byte
		expected string
		wantErr  bool
	}{
		{
			name:     "CSV format",
			data:     []byte("id;url;date\n1;example.com;2023-01-01"),
			expected: "csv",
			wantErr:  false,
		},
		{
			name:     "ZIP format",
			data:     []byte{0x50, 0x4B, 0x03, 0x04, 0x00, 0x00},
			expected: "zip",
			wantErr:  false,
		},
		{
			name:     "Empty data",
			data:     []byte{},
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Unknown format",
			data:     []byte("random binary data"),
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format, err := parser.detectFormat(tt.data)

			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if format != tt.expected {
				t.Errorf("expected format %q, got %q", tt.expected, format)
			}
		})
	}
}

func TestParser_parseCSVText(t *testing.T) {
	parser := NewParser()

	csvData := `id;url;date
1;example.com;2023-01-01
2;*.test.com;2023-01-02
3;192.168.1.1;2023-01-03
4;blocked.com/path;2023-01-04
5;https://secure.com;2023-01-05`

	registry, err := parser.parseCSVText(csvData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if registry == nil {
		t.Fatal("registry is nil")
	}

	if registry.Size() == 0 {
		t.Error("registry should not be empty")
	}

	// Verify entries were created
	domainEntries := registry.GetEntriesByType(domain.BlockingTypeDomain)
	wildcardEntries := registry.GetEntriesByType(domain.BlockingTypeWildcard)
	ipEntries := registry.GetEntriesByType(domain.BlockingTypeIP)
	urlEntries := registry.GetEntriesByType(domain.BlockingTypeURLPath)

	if len(domainEntries) == 0 {
		t.Error("expected domain entries")
	}
	if len(wildcardEntries) == 0 {
		t.Error("expected wildcard entries")
	}
	if len(ipEntries) == 0 {
		t.Error("expected IP entries")
	}
	if len(urlEntries) == 0 {
		t.Error("expected URL path entries")
	}
}

func TestParser_categorizeEntry(t *testing.T) {
	parser := NewParser()
	registry := domain.NewRegistry()

	tests := []struct {
		name         string
		entry        string
		expectedType domain.BlockingType
		expectError  bool
	}{
		{
			name:         "Domain entry",
			entry:        "example.com",
			expectedType: domain.BlockingTypeDomain,
			expectError:  false,
		},
		{
			name:         "Wildcard entry",
			entry:        "*.example.com",
			expectedType: domain.BlockingTypeWildcard,
			expectError:  false,
		},
		{
			name:         "IPv4 entry",
			entry:        "192.168.1.1",
			expectedType: domain.BlockingTypeIP,
			expectError:  false,
		},
		{
			name:         "IPv6 entry",
			entry:        "2001:db8::1",
			expectedType: domain.BlockingTypeIP,
			expectError:  false,
		},
		{
			name:         "URL path entry",
			entry:        "example.com/blocked/path",
			expectedType: domain.BlockingTypeURLPath,
			expectError:  false,
		},
		{
			name:         "Domain with protocol",
			entry:        "https://example.com",
			expectedType: domain.BlockingTypeDomain,
			expectError:  false,
		},
		{
			name:         "Invalid entry",
			entry:        "invalid..domain",
			expectedType: domain.BlockingTypeUnknown,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialSize := registry.Size()
			err := parser.categorizeEntry(tt.entry, registry)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError {
				// Verify entry was added
				if registry.Size() != initialSize+1 {
					t.Error("entry was not added to registry")
				}

				// Verify entry type (check the last added entry)
				if registry.Size() > 0 {
					lastEntry := registry.Entries[registry.Size()-1]
					if lastEntry.Type != tt.expectedType {
						t.Errorf("expected type %v, got %v", tt.expectedType, lastEntry.Type)
					}
				}
			}
		})
	}
}

func TestParser_stripProtocol(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		input    string
		expected string
	}{
		{"https://example.com", "example.com"},
		{"http://example.com", "example.com"},
		{"ftp://example.com", "example.com"},
		{"example.com", "example.com"},
		{"HTTPS://EXAMPLE.COM", "EXAMPLE.COM"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parser.stripProtocol(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestParser_isIPAddress(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		input    string
		expected bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"255.255.255.255", true},
		{"2001:db8::1", true},
		{"::1", true},
		{"::", true},
		{"example.com", false},
		{"192.168.1.256", false},
		{"not.an.ip", false},
		{"192.168.1.1:8080", true}, // Should handle port
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parser.isIPAddress(tt.input)
			if result != tt.expected {
				t.Errorf("input %q: expected %v, got %v", tt.input, tt.expected, result)
			}
		})
	}
}

func TestParser_parseCSVText_EmptyData(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name    string
		csvData string
		wantErr bool
	}{
		{
			name:    "Empty CSV",
			csvData: "",
			wantErr: true,
		},
		{
			name:    "Header only",
			csvData: "id;url;date",
			wantErr: true,
		},
		{
			name:    "Invalid entries only",
			csvData: "id;url;date\n1;;2023-01-01\n2;invalid..domain;2023-01-02",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.parseCSVText(tt.csvData)
			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestParser_parseCSVText_MultipleEntries(t *testing.T) {
	parser := NewParser()

	csvData := `id;url;date
1;example1.com|example2.com;2023-01-01
2;*.wildcard.com|blocked.com;2023-01-02`

	registry, err := parser.parseCSVText(csvData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have multiple entries from pipe-separated values
	if registry.Size() < 4 {
		t.Errorf("expected at least 4 entries, got %d", registry.Size())
	}
}

func BenchmarkParser_parseCSVText(b *testing.B) {
	parser := NewParser()

	// Generate test data
	var csvData strings.Builder
	csvData.WriteString("id;url;date\n")
	for i := 0; i < 1000; i++ {
		csvData.WriteString("1;example.com;2023-01-01\n")
		csvData.WriteString("2;*.test.com;2023-01-02\n")
		csvData.WriteString("3;192.168.1.1;2023-01-03\n")
	}

	data := csvData.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.parseCSVText(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
