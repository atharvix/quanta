// Package handlers implements HTTP request handlers and JSON serialization helpers.
package handlers

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse represents standard structured error responses.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ValidationErrorResponse provides field-level validation breakdowns for HTTP 400 responses.
type ValidationErrorResponse struct {
	Error   string   `json:"error"`
	Details []string `json:"details"`
}

// WriteJSON sends a JSON response with the given HTTP status code.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// WriteError sends a standard JSON error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message})
}

// WriteValidationError sends a structured validation failure response with field-level details.
func WriteValidationError(w http.ResponseWriter, details []string) {
	WriteJSON(w, http.StatusBadRequest, ValidationErrorResponse{
		Error:   "validation failed",
		Details: details,
	})
}
