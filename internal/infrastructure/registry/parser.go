package registry

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
	"golang.org/x/text/encoding/charmap"
)

// Parser handles parsing of registry data in various formats
type Parser struct {
	// Regex patterns for validation
	domainPattern   *regexp.Regexp
	wildcardPattern *regexp.Regexp
	ipv4Pattern     *regexp.Regexp
	ipv6Pattern     *regexp.Regexp
}

// NewParser creates a new registry parser
func NewParser() *Parser {
	return &Parser{
		domainPattern:   regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`),
		wildcardPattern: regexp.MustCompile(`^\*\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`),
		ipv4Pattern:     regexp.MustCompile(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`),
		ipv6Pattern:     regexp.MustCompile(`^([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$|^::1$|^::$`),
	}
}

// Parse parses registry data and returns a Registry
func (p *Parser) Parse(data []byte) (*domain.Registry, error) {
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	// Detect format by checking magic bytes
	format, err := p.detectFormat(data)
	if err != nil {
		return nil, fmt.Errorf("detecting format: %w", err)
	}

	switch format {
	case "csv":
		return p.parseCSV(data)
	case "zip":
		return p.parseZIP(data)
	default:
		return nil, NewParsingError(format, ErrUnsupportedFormat)
	}
}

// detectFormat detects the data format based on content
func (p *Parser) detectFormat(data []byte) (string, error) {
	if len(data) < 4 {
		return "", ErrInvalidFormat
	}

	// Check for ZIP magic bytes
	if bytes.HasPrefix(data, []byte{0x50, 0x4B, 0x03, 0x04}) ||
		bytes.HasPrefix(data, []byte{0x50, 0x4B, 0x05, 0x06}) ||
		bytes.HasPrefix(data, []byte{0x50, 0x4B, 0x07, 0x08}) {
		return "zip", nil
	}

	// Check if it looks like CSV (contains semicolons or commas)
	sample := string(data[:min(1024, len(data))])
	if strings.Contains(sample, ";") || strings.Contains(sample, ",") {
		return "csv", nil
	}

	return "", ErrUnsupportedFormat
}

// parseZIP extracts and parses CSV files from ZIP archive
func (p *Parser) parseZIP(data []byte) (*domain.Registry, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, NewParsingError("zip", fmt.Errorf("opening ZIP: %w", err))
	}

	// Look for CSV files in the archive
	for _, file := range reader.File {
		if strings.HasSuffix(strings.ToLower(file.Name), ".csv") {
			rc, err := file.Open()
			if err != nil {
				continue
			}

			csvData, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			// Try to parse this CSV file
			registry, err := p.parseCSV(csvData)
			if err == nil {
				return registry, nil
			}
		}
	}

	return nil, NewParsingError("zip", fmt.Errorf("no valid CSV found in archive"))
}

// parseCSV parses CSV format registry data
func (p *Parser) parseCSV(data []byte) (*domain.Registry, error) {
	// Try different encodings
	encodings := []func([]byte) (string, error){
		func(d []byte) (string, error) { return string(d), nil },         // UTF-8
		func(d []byte) (string, error) { return p.decodeWindows1251(d) }, // Windows-1251
	}

	var lastErr error
	for _, decode := range encodings {
		text, err := decode(data)
		if err != nil {
			lastErr = err
			continue
		}

		registry, err := p.parseCSVText(text)
		if err == nil {
			return registry, nil
		}
		lastErr = err
	}

	return nil, NewParsingError("csv", lastErr)
}

// decodeWindows1251 decodes Windows-1251 encoded data
func (p *Parser) decodeWindows1251(data []byte) (string, error) {
	decoder := charmap.Windows1251.NewDecoder()
	result, err := decoder.Bytes(data)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

// parseCSVText parses CSV text content
func (p *Parser) parseCSVText(text string) (*domain.Registry, error) {
	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma = ';' // RKN registry uses semicolon separator
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	registry := domain.NewRegistry()
	registry.Source = "RKN Registry"

	lineNum := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, NewParsingErrorWithPosition("csv", lineNum, 0, err)
		}

		lineNum++

		// Skip header or empty lines
		if lineNum == 1 || len(record) == 0 {
			continue
		}

		if err := p.parseCSVRecord(record, registry); err != nil {
			// Log parse errors but continue processing
			continue
		}
	}

	if registry.Size() == 0 {
		return nil, NewParsingError("csv", fmt.Errorf("no valid entries found"))
	}

	return registry, nil
}

// parseCSVRecord parses a single CSV record
func (p *Parser) parseCSVRecord(record []string, registry *domain.Registry) error {
	if len(record) < 2 {
		return fmt.Errorf("insufficient columns")
	}

	// Common RKN CSV format: [id, url, date, ...]
	// We're primarily interested in the URL field
	urlField := strings.TrimSpace(record[1])
	if urlField == "" {
		return fmt.Errorf("empty URL field")
	}

	// Parse different types of entries
	entries := strings.Split(urlField, "|")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if err := p.categorizeEntry(entry, registry); err != nil {
			// Continue processing even if one entry fails
			continue
		}
	}

	return nil
}

// categorizeEntry categorizes a single entry into the appropriate registry section
func (p *Parser) categorizeEntry(entry string, registry *domain.Registry) error {
	originalEntry := entry
	entry = strings.ToLower(entry)

	// Remove protocol if present
	entry = p.stripProtocol(entry)

	var blockingType domain.BlockingType
	var value string

	// Check for wildcard domains
	if strings.HasPrefix(entry, "*.") {
		if p.wildcardPattern.MatchString(entry) {
			blockingType = domain.BlockingTypeWildcard
			value = entry
		} else {
			return fmt.Errorf("invalid wildcard format: %s", entry)
		}
	} else if p.isIPAddress(entry) {
		// Check for IP addresses
		blockingType = domain.BlockingTypeIP
		value = entry
	} else if strings.Contains(entry, "/") {
		// Check for URLs with paths
		blockingType = domain.BlockingTypeURLPath
		value = entry
	} else if p.domainPattern.MatchString(entry) {
		// Default to domain entry
		blockingType = domain.BlockingTypeDomain
		value = entry
	} else {
		return fmt.Errorf("unrecognized entry format: %s", entry)
	}

	// Create registry entry
	registryEntry, err := domain.NewRegistryEntry(blockingType, value)
	if err != nil {
		return fmt.Errorf("creating registry entry for %s: %w", originalEntry, err)
	}

	// Add additional context if available from CSV
	if len(registry.Entries) > 0 {
		registryEntry.ID = fmt.Sprintf("rkn_%d", len(registry.Entries))
	}

	return registry.AddEntry(registryEntry)
}

// stripProtocol removes protocol prefix from URL
func (p *Parser) stripProtocol(entry string) string {
	protocols := []string{"https://", "http://", "ftp://"}
	for _, protocol := range protocols {
		if strings.HasPrefix(strings.ToLower(entry), protocol) {
			return entry[len(protocol):]
		}
	}
	return entry
}

// isIPAddress checks if the entry is an IP address
func (p *Parser) isIPAddress(entry string) bool {
	// Remove port if present
	if strings.Contains(entry, ":") && !strings.Contains(entry, "::") {
		parts := strings.Split(entry, ":")
		if len(parts) == 2 {
			entry = parts[0]
		}
	}

	// Use Go's built-in IP parsing
	ip := net.ParseIP(entry)
	return ip != nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
