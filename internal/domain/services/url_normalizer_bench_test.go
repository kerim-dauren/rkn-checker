package services

import (
	"testing"
)

func BenchmarkURLNormalizer_Normalize_Simple(b *testing.B) {
	normalizer := NewURLNormalizer()
	url := "https://example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = normalizer.Normalize(url)
	}
}

func BenchmarkURLNormalizer_Normalize_Complex(b *testing.B) {
	normalizer := NewURLNormalizer()
	url := "HTTPS://WWW.EXAMPLE.COM:8080/path/to/resource?query=value&param=1#fragment"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = normalizer.Normalize(url)
	}
}

func BenchmarkURLNormalizer_Normalize_IDN(b *testing.B) {
	normalizer := NewURLNormalizer()
	url := "https://тест.рф"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = normalizer.Normalize(url)
	}
}