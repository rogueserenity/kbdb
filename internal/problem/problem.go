// Package problem writes RFC 9457 Problem Details HTTP responses.
package problem

import (
	"encoding/json"
	"net/http"
)

type body struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// Write writes an RFC 9457 application/problem+json response with the given
// status, title, and detail. problemType should be a stable URI identifying
// the problem category (e.g. "https://mykeebs.info/errors/internal-error").
func Write(w http.ResponseWriter, status int, problemType, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body{
		Type:   problemType,
		Title:  title,
		Status: status,
		Detail: detail,
	})
}

// NotFound writes a 404 Problem response.
func NotFound(w http.ResponseWriter, detail string) {
	Write(w, http.StatusNotFound, "https://mykeebs.info/errors/not-found", "Not Found", detail)
}

// Internal writes a 500 Problem response.
func Internal(w http.ResponseWriter, detail string) {
	Write(w, http.StatusInternalServerError, "https://mykeebs.info/errors/internal-error", "Internal Server Error", detail)
}

// Forbidden writes a 403 Problem response.
func Forbidden(w http.ResponseWriter, detail string) {
	Write(w, http.StatusForbidden, "https://mykeebs.info/errors/forbidden", "Forbidden", detail)
}

// Conflict writes a 409 Problem response.
func Conflict(w http.ResponseWriter, detail string) {
	Write(w, http.StatusConflict, "https://mykeebs.info/errors/conflict", "Conflict", detail)
}

// BadRequest writes a 400 Problem response.
func BadRequest(w http.ResponseWriter, detail string) {
	Write(w, http.StatusBadRequest, "https://mykeebs.info/errors/bad-request", "Bad Request", detail)
}
