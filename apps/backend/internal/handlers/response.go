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
