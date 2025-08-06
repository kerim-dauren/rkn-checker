package rest

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/kerim-dauren/rkn-checker/internal/application"
	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

type Handler struct {
	blockingService application.BlockingChecker
}

func NewHandler(blockingService application.BlockingChecker) *Handler {
	return &Handler{
		blockingService: blockingService,
	}
}

func (h *Handler) CheckURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req CheckURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if strings.TrimSpace(req.URL) == "" {
		WriteErrorResponse(w, http.StatusBadRequest, "URL is required")
		return
	}

	result, err := h.blockingService.CheckURL(r.Context(), req.URL)
	if err != nil {
		switch err {
		case domain.ErrEmptyURL:
			WriteErrorResponse(w, http.StatusBadRequest, "URL is empty")
			return
		case domain.ErrInvalidURL:
			WriteErrorResponse(w, http.StatusBadRequest, "Invalid URL format")
			return
		default:
			slog.Error("Failed to check URL", "error", err)
			WriteErrorResponse(w, http.StatusInternalServerError, "Internal server error")
			return
		}
	}

	response := CheckURLResponse{
		Blocked:       result.IsBlocked,
		NormalizedURL: result.NormalizedURL,
	}

	if result.IsBlocked {
		response.Reason = result.Reason.String()
		if result.Rule != nil {
			response.Match = result.Rule.Pattern
		}
	}

	WriteJSONResponse(w, http.StatusOK, response)
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	stats, err := h.blockingService.GetStats(r.Context())
	if err != nil {
		slog.Error("Failed to get stats", "error", err)
		WriteErrorResponse(w, http.StatusInternalServerError, "Failed to get statistics")
		return
	}

	response := StatsResponse{
		TotalEntries:    stats.TotalEntries,
		DomainEntries:   stats.DomainEntries,
		WildcardEntries: stats.WildcardEntries,
		IPEntries:       stats.IPEntries,
		URLPatterns:     stats.URLPatterns,
		LastUpdate:      stats.LastUpdate,
		Version:         stats.Version,
	}

	WriteJSONResponse(w, http.StatusOK, response)
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	response := HealthResponse{
		Status:  "healthy",
		Message: "Service is operational",
	}

	WriteJSONResponse(w, http.StatusOK, response)
}
