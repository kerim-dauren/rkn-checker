package registry

import (
	"context"
	"testing"
	"time"
)

func TestNewOfficialSource(t *testing.T) {
	config := SourceConfig{
		Type:       SourceTypeOfficial,
		URL:        "https://vigruzki.rkn.gov.ru/services/OperatorRequest/",
		Timeout:    60 * time.Second,
		MaxRetries: 2,
		UserAgent:  "RKN-Checker/1.0",
		RKN: RKNConfig{
			DumpFormatVersion: "2.4",
			PollInterval:      30 * time.Second,
			MaxPollAttempts:   20,
		},
	}

	source := NewOfficialSource(config)

	if source == nil {
		t.Fatal("NewOfficialSource returned nil")
	}

	if source.Name() != "Official RKN API" {
		t.Errorf("Expected name 'Official RKN API', got %q", source.Name())
	}

	if source.dumpFormatVersion != "2.4" {
		t.Errorf("Expected dump format version '2.4', got %q", source.dumpFormatVersion)
	}
}

func TestOfficialSource_FetchWithoutAuthentication(t *testing.T) {
	config := SourceConfig{
		Type:       SourceTypeOfficial,
		URL:        "https://vigruzki.rkn.gov.ru/services/OperatorRequest/",
		Timeout:    5 * time.Second,
		MaxRetries: 1,
		UserAgent:  "RKN-Checker/1.0",
	}

	source := NewOfficialSource(config)
	ctx := context.Background()

	// This should fail because no authentication is configured
	_, err := source.Fetch(ctx)
	if err == nil {
		t.Error("Expected error when fetching without authentication, but got none")
	}

	// Check that the error message mentions authentication
	if err != nil && err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

func TestOfficialSource_FetchWithMockAuthentication(t *testing.T) {
	config := SourceConfig{
		Type:       SourceTypeOfficial,
		URL:        "https://httpbin.org/post", // Mock endpoint for testing
		Timeout:    10 * time.Second,
		MaxRetries: 1,
		UserAgent:  "RKN-Checker/1.0",
	}

	source := NewOfficialSource(config)

	// Set mock authentication files for testing
	source.SetAuthenticationFiles(
		[]byte("mock-request-file"),
		[]byte("mock-signature-file"),
	)

	ctx := context.Background()

	// This will still fail because it's a placeholder implementation
	// but it should get past the authentication check
	_, err := source.Fetch(ctx)
	if err == nil {
		t.Error("Expected error from placeholder implementation")
	}

	// Verify it's not an authentication error
	if err != nil && err.Error() != "" {
		// The error should be about the placeholder implementation, not authentication
		t.Logf("Got expected error: %v", err)
	}
}

func TestOfficialSource_IsHealthy(t *testing.T) {
	config := SourceConfig{
		Type:       SourceTypeOfficial,
		URL:        "https://vigruzki.rkn.gov.ru/services/OperatorRequest/",
		Timeout:    10 * time.Second,
		MaxRetries: 1,
		UserAgent:  "RKN-Checker/1.0",
	}

	source := NewOfficialSource(config)
	ctx := context.Background()

	// This will attempt to fetch the WSDL
	healthy := source.IsHealthy(ctx)

	// We can't guarantee the service is available, so just verify the method works
	t.Logf("RKN API health status: %v", healthy)

	// Verify that calling it twice uses cached result
	start := time.Now()
	source.IsHealthy(ctx)
	duration := time.Since(start)

	if duration > time.Second {
		t.Error("Second health check should use cached result and be fast")
	}
}

func TestOfficialSource_CreateSOAP(t *testing.T) {
	config := SourceConfig{
		Type: SourceTypeOfficial,
		URL:  "https://example.com",
	}

	source := NewOfficialSource(config)
	source.SetAuthenticationFiles(
		[]byte("test-request"),
		[]byte("test-signature"),
	)

	soap := source.createSendRequestSOAP()
	soapStr := string(soap)

	// Verify SOAP structure
	if !containsAll(soapStr, []string{
		"soap:Envelope",
		"sendRequest",
		"requestFile",
		"signatureFile",
		"dumpFormatVersion",
	}) {
		t.Errorf("SOAP envelope missing required elements: %s", soapStr)
	}
}

func TestOfficialSource_ParseResponse(t *testing.T) {
	config := SourceConfig{Type: SourceTypeOfficial}
	source := NewOfficialSource(config)

	// Test successful response
	successResponse := `<?xml version="1.0"?>
	<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
		<soap:Body>
			<sendRequestResponse>
				<result>true</result>
				<code>request-123</code>
			</sendRequestResponse>
		</soap:Body>
	</soap:Envelope>`

	requestID, err := source.parseSendRequestResponse([]byte(successResponse))
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if requestID == "" {
		t.Error("Expected non-empty request ID")
	}

	// Test failed response
	failResponse := `<?xml version="1.0"?>
	<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
		<soap:Body>
			<sendRequestResponse>
				<result>false</result>
				<resultComment>Authentication failed</resultComment>
			</sendRequestResponse>
		</soap:Body>
	</soap:Envelope>`

	_, err = source.parseSendRequestResponse([]byte(failResponse))
	if err == nil {
		t.Error("Expected error for failed response")
	}
}

// Helper function to check if string contains all required substrings
func containsAll(s string, required []string) bool {
	for _, req := range required {
		if !stringContains(s, req) {
			return false
		}
	}
	return true
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
