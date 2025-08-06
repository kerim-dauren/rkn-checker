package common

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
}

type ErrorInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

func WriteSuccess(w http.ResponseWriter, data interface{}) {
	response := Response{
		Success: true,
		Data:    data,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func WriteError(w http.ResponseWriter, statusCode int, message string) {
	response := Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    statusCode,
			Message: message,
			Type:    http.StatusText(statusCode),
		},
	}
	writeJSONResponse(w, statusCode, response)
}

func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
