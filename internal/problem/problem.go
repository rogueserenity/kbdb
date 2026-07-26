// Package problem writes RFC 9457 Problem Details HTTP responses.
package problem

import (
	"encoding/json"
	"net/http"
)

type body struct {
	Type          string         `json:"type"`
	Title         string         `json:"title"`
	Status        int            `json:"status"`
	Detail        string         `json:"detail,omitempty"`
	InvalidParams []InvalidParam `json:"invalid_params,omitempty"`
}

// InvalidParam is one field-level violation reported in a validation
// problem's invalid_params member (RFC 9457 §3.2).
type InvalidParam struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// Write writes an RFC 9457 application/problem+json response with the given
// status, title, and detail. problemType should be a stable URI identifying
// the problem category (e.g. "https://mykeebs.info/errors/internal-error").
func Write(w http.ResponseWriter, status int, problemType, title, detail string) {
	writeBody(w, status, body{
		Type:   problemType,
		Title:  title,
		Status: status,
		Detail: detail,
	})
}

// ValidationFailed writes a 400 Problem response listing every field-level
// violation in invalidParams, per RFC 9457 §3.2's invalid_params member.
func ValidationFailed(w http.ResponseWriter, detail string, invalidParams []InvalidParam) {
	writeBody(w, http.StatusBadRequest, body{
		Type:          "https://mykeebs.info/errors/bad-request",
		Title:         "Bad Request",
		Status:        http.StatusBadRequest,
		Detail:        detail,
		InvalidParams: invalidParams,
	})
}

func writeBody(w http.ResponseWriter, status int, b body) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(b)
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

// Unauthorized writes a 401 Problem response.
func Unauthorized(w http.ResponseWriter, detail string) {
	Write(w, http.StatusUnauthorized, "https://mykeebs.info/errors/unauthorized", "Unauthorized", detail)
}

// Conflict writes a 409 Problem response.
func Conflict(w http.ResponseWriter, detail string) {
	Write(w, http.StatusConflict, "https://mykeebs.info/errors/conflict", "Conflict", detail)
}

// BadRequest writes a 400 Problem response.
func BadRequest(w http.ResponseWriter, detail string) {
	Write(w, http.StatusBadRequest, "https://mykeebs.info/errors/bad-request", "Bad Request", detail)
}
