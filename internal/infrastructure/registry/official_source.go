package registry

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// OfficialSource implements Source interface for official RKN API
// This uses the SOAP-based API at https://vigruzki.rkn.gov.ru/services/OperatorRequest/
type OfficialSource struct {
	client *http.Client
	config SourceConfig

	// Health tracking (protected by mutex)
	healthMu   sync.RWMutex
	lastHealth time.Time
	healthy    bool

	// Authentication fields (would be populated from config in real implementation)
	requestFile       []byte // Base64 encoded request file
	signatureFile     []byte // Base64 encoded signature
	emchdFile         []byte // Base64 encoded power of attorney (optional)
	emchdFileName     string
	emchdSignature    []byte
	dumpFormatVersion string
	// Testing mode - if true, skip SOAP and fetch directly as CSV
	testMode bool
}

// NewOfficialSource creates a new official RKN API source
func NewOfficialSource(config SourceConfig) *OfficialSource {
	// Configure TLS for SOAP client
	tlsConfig := &tls.Config{
		InsecureSkipVerify: config.RKN.InsecureSkipVerify,
	}

	// Load client certificates if configured
	if config.RKN.CertFilePath != "" && config.RKN.KeyFilePath != "" {
		// In real implementation, load cert/key files here
		// cert, err := tls.LoadX509KeyPair(config.RKN.CertFilePath, config.RKN.KeyFilePath)
		// if err == nil {
		//     tlsConfig.Certificates = []tls.Certificate{cert}
		// }
	}

	source := &OfficialSource{
		client: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				TLSClientConfig:    tlsConfig,
				MaxIdleConns:       5,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
			},
		},
		config:            config,
		healthy:           true,
		dumpFormatVersion: config.RKN.DumpFormatVersion,
	}

	// Set default dump format version if not specified
	if source.dumpFormatVersion == "" {
		source.dumpFormatVersion = "2.4"
	}

	// Load authentication files if configured
	if err := source.loadAuthenticationFiles(); err != nil {
		// In real implementation, might want to log this error
		// but still return the source (it will fail when trying to fetch)
	}

	return source
}

// Name returns the source name
func (o *OfficialSource) Name() string {
	return "Official RKN API"
}

// Fetch retrieves registry data from official RKN API
// Note: This may be blocked when accessed from Germany
func (o *OfficialSource) Fetch(ctx context.Context) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt < o.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter
			backoff := time.Duration(attempt*attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		data, err := o.fetchOnce(ctx)
		if err == nil {
			o.healthMu.Lock()
			o.healthy = true
			o.lastHealth = time.Now()
			o.healthMu.Unlock()
			return data, nil
		}

		lastErr = err
	}

	o.healthMu.Lock()
	o.healthy = false
	o.healthMu.Unlock()
	return nil, NewSourceError(o.Name(), "fetch", lastErr)
}

// fetchOnce performs a single fetch attempt using SOAP API or direct HTTP (test mode)
func (o *OfficialSource) fetchOnce(ctx context.Context) ([]byte, error) {
	// If in test mode, use direct HTTP GET instead of SOAP
	if o.testMode {
		return o.fetchDirect(ctx)
	}

	// Step 1: Send request to get registry data
	requestID, err := o.sendSOAPRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("sending SOAP request: %w", err)
	}

	// Step 2: Poll for results (simplified implementation)
	// In real implementation, would poll getResult until ready
	data, err := o.getSOAPResult(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("getting SOAP result: %w", err)
	}

	return data, nil
}

// fetchDirect performs a direct HTTP GET (for testing with mock servers)
func (o *OfficialSource) fetchDirect(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", o.config.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", o.config.UserAgent)
	req.Header.Set("Accept", "text/csv, application/xml")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	return data, nil
}

// sendSOAPRequest sends a SOAP request for registry data
func (o *OfficialSource) sendSOAPRequest(ctx context.Context) (string, error) {
	// Check if authentication is properly configured
	if len(o.requestFile) == 0 || len(o.signatureFile) == 0 {
		return "", fmt.Errorf("RKN API requires authentication: request file and digital signature must be configured")
	}

	// Create SOAP envelope for sendRequest
	soapBody := o.createSendRequestSOAP()

	req, err := http.NewRequestWithContext(ctx, "POST", o.config.URL, bytes.NewReader(soapBody))
	if err != nil {
		return "", fmt.Errorf("creating SOAP request: %w", err)
	}

	// Set SOAP headers
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "sendRequest")
	req.Header.Set("User-Agent", o.config.UserAgent)

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("SOAP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("SOAP HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading SOAP response: %w", err)
	}

	// Parse SOAP response to extract request ID
	requestID, err := o.parseSendRequestResponse(responseData)
	if err != nil {
		return "", fmt.Errorf("parsing SOAP response: %w", err)
	}

	return requestID, nil
}

// getSOAPResult retrieves the result for a given request ID
func (o *OfficialSource) getSOAPResult(ctx context.Context, requestID string) ([]byte, error) {
	// This is a simplified implementation
	// Real implementation would poll getResult method until data is ready
	return nil, fmt.Errorf("RKN API integration requires proper authentication setup and request/response handling - this is a placeholder implementation")
}

// createSendRequestSOAP creates SOAP envelope for sendRequest method
func (o *OfficialSource) createSendRequestSOAP() []byte {
	// This is a simplified SOAP envelope structure
	// Real implementation would need proper Base64 encoding of files
	soapEnvelope := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <sendRequest xmlns="http://vigruzki.rkn.gov.ru/services/OperatorRequest/">
      <requestFile>%s</requestFile>
      <signatureFile>%s</signatureFile>
      <dumpFormatVersion>%s</dumpFormatVersion>
    </sendRequest>
  </soap:Body>
</soap:Envelope>`,
		string(o.requestFile),
		string(o.signatureFile),
		o.dumpFormatVersion)

	return []byte(soapEnvelope)
}

// parseSendRequestResponse parses SOAP response to extract request ID
func (o *OfficialSource) parseSendRequestResponse(data []byte) (string, error) {
	// Simple XML parsing to extract response data
	// Real implementation would use proper XML unmarshaling
	response := string(data)

	// Look for success indicators in SOAP response
	if strings.Contains(response, "<result>true</result>") {
		// Extract request ID (simplified - would need proper XML parsing)
		return "placeholder-request-id", nil
	}

	// Extract error message if present
	if strings.Contains(response, "<result>false</result>") {
		return "", fmt.Errorf("RKN API request failed - check authentication and request format")
	}

	return "", fmt.Errorf("invalid SOAP response format")
}

// IsHealthy checks if the SOAP service is currently available
func (o *OfficialSource) IsHealthy(ctx context.Context) bool {
	// Return cached health status if checked recently
	o.healthMu.RLock()
	lastHealth := o.lastHealth
	healthy := o.healthy
	o.healthMu.RUnlock()

	if time.Since(lastHealth) < 5*time.Minute {
		return healthy
	}

	// For SOAP services, check WSDL availability
	wsdlURL := strings.TrimSuffix(o.config.URL, "/") + "?wsdl"

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", wsdlURL, nil)
	if err != nil {
		o.healthMu.Lock()
		o.healthy = false
		o.healthMu.Unlock()
		return false
	}

	req.Header.Set("User-Agent", o.config.UserAgent)

	resp, err := o.client.Do(req)
	if err != nil {
		o.healthMu.Lock()
		o.healthy = false
		o.healthMu.Unlock()
		return false
	}
	defer resp.Body.Close()

	// Check if WSDL is available (indicates SOAP service is running)
	o.healthMu.Lock()
	o.healthy = resp.StatusCode == http.StatusOK
	o.lastHealth = time.Now()
	healthResult := o.healthy
	o.healthMu.Unlock()

	return healthResult
}

// SOAP response structures for proper XML unmarshaling
type SOAPEnvelope struct {
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Envelope"`
	Body    SOAPBody `xml:"Body"`
}

type SOAPBody struct {
	SendRequestResponse *SendRequestResponse `xml:"sendRequestResponse,omitempty"`
	GetResultResponse   *GetResultResponse   `xml:"getResultResponse,omitempty"`
}

type SendRequestResponse struct {
	Result        bool   `xml:"result"`
	ResultComment string `xml:"resultComment,omitempty"`
	Code          string `xml:"code,omitempty"`
}

type GetResultResponse struct {
	Result     bool   `xml:"result"`
	ResultCode int    `xml:"resultCode,omitempty"`
	Zip        []byte `xml:"zip,omitempty"`
}

// loadAuthenticationFiles loads authentication files from configured paths
func (o *OfficialSource) loadAuthenticationFiles() error {
	// Load request file if configured
	if o.config.RKN.RequestFilePath != "" {
		// In real implementation, read file and encode to base64
		// data, err := os.ReadFile(o.config.RKN.RequestFilePath)
		// if err != nil {
		//     return fmt.Errorf("loading request file: %w", err)
		// }
		// o.requestFile = []byte(base64.StdEncoding.EncodeToString(data))
	}

	// Load signature file if configured
	if o.config.RKN.SignatureFilePath != "" {
		// In real implementation, read signature file and encode to base64
		// data, err := os.ReadFile(o.config.RKN.SignatureFilePath)
		// if err != nil {
		//     return fmt.Errorf("loading signature file: %w", err)
		// }
		// o.signatureFile = []byte(base64.StdEncoding.EncodeToString(data))
	}

	// Load EMCHD files if configured
	if o.config.RKN.EMCHDFilePath != "" {
		// In real implementation, load power of attorney file
		// Similar to above...
		o.emchdFileName = o.config.RKN.EMCHDFilePath
	}

	return nil
}

// SetAuthenticationFiles allows setting authentication data directly (for testing)
func (o *OfficialSource) SetAuthenticationFiles(requestFile, signatureFile []byte) {
	o.requestFile = requestFile
	o.signatureFile = signatureFile
}

// SetTestMode enables or disables test mode (direct HTTP instead of SOAP)
func (o *OfficialSource) SetTestMode(enabled bool) {
	o.testMode = enabled
}

// GetOfficialSource returns the OfficialSource if the source is of that type
func GetOfficialSource(source Source) (*OfficialSource, bool) {
	if officialSource, ok := source.(*OfficialSource); ok {
		return officialSource, true
	}
	return nil, false
}
