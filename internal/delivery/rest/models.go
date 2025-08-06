package rest

import (
	"encoding/json"
	"net/http"
)

type CheckURLRequest struct {
	URL string `json:"url"`
}

type CheckURLResponse struct {
	Blocked       bool   `json:"blocked"`
	NormalizedURL string `json:"normalized_url"`
	Reason        string `json:"reason,omitempty"`
	Match         string `json:"match,omitempty"`
}

type StatsResponse struct {
	TotalEntries    int64  `json:"total_entries"`
	DomainEntries   int64  `json:"domain_entries"`
	WildcardEntries int64  `json:"wildcard_entries"`
	IPEntries       int64  `json:"ip_entries"`
	URLPatterns     int64  `json:"url_patterns"`
	LastUpdate      string `json:"last_update"`
	Version         string `json:"version"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func WriteJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func WriteErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := ErrorResponse{
		Error:   http.StatusText(statusCode),
		Code:    statusCode,
		Message: message,
	}
	WriteJSONResponse(w, statusCode, response)
}