// Package handlers holds route handler logic, separate from middleware and
// from router wiring.
package handlers

import (
	"encoding/json"
	"net/http"
)

// Ping responds with a trivial 200 to confirm the request reached real
// handler logic (as opposed to the API Gateway authorizer or a placeholder).
func Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
