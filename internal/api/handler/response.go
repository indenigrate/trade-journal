package handler

import (
	"encoding/json"
	"net/http"

	"github.com/onesine/nevup-backend/internal/api/middleware"
)

// ErrorResponse is the canonical error response shape.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	TraceID string `json:"traceId"`
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func respondError(w http.ResponseWriter, r *http.Request, status int, errorCode, message string) {
	traceID := middleware.TraceIDFromContext(r.Context())
	respondJSON(w, status, ErrorResponse{
		Error:   errorCode,
		Message: message,
		TraceID: traceID,
	})
}

func respondForbidden(w http.ResponseWriter, r *http.Request) {
	respondError(w, r, http.StatusForbidden, "FORBIDDEN", "Cross-tenant access denied.")
}

func respondNotFound(w http.ResponseWriter, r *http.Request, message string) {
	respondError(w, r, http.StatusNotFound, "NOT_FOUND", message)
}

func respondBadRequest(w http.ResponseWriter, r *http.Request, message string) {
	respondError(w, r, http.StatusBadRequest, "BAD_REQUEST", message)
}
