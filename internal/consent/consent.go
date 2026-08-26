// Package consent hosts the Stytch consent/authorization page kbdb serves
// at GET /authorize, so MCP clients can authenticate against kbdb
// standalone without depending on mykeebs-web being deployed.
package consent

import (
	"bytes"
	_ "embed"
	"html/template"
	"log"
	"net/http"
)

//go:embed authorize.html
var authorizeHTML string

var authorizeTemplate = template.Must(template.New("authorize.html").Parse(authorizeHTML))

// templateData fields are rendered by html/template into a <script> block -
// its context-aware escaping renders them as safe JS string literals
// (including escaping a </script> breakout attempt), so fields are passed
// raw, not pre-quoted.
type templateData struct {
	StytchPublicToken string
	OIDCIssuerBaseURL string
}

// Handler serves GET /authorize: kbdb's Stytch consent/login page.
func Handler(stytchPublicToken, oidcIssuerBaseURL string) http.Handler {
	var buf bytes.Buffer
	data := templateData{
		StytchPublicToken: stytchPublicToken,
		OIDCIssuerBaseURL: oidcIssuerBaseURL,
	}
	if err := authorizeTemplate.Execute(&buf, data); err != nil {
		log.Fatalf("consent: rendering authorize.html: %v", err)
	}
	page := buf.Bytes()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
}
