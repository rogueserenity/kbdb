// Package consent hosts the Stytch consent/authorization page kbdb serves
// at GET /authorize. Stytch's Connected Apps has no fully-hosted login
// domain the way WorkOS's AuthKit did - the <StytchUI>/<StytchIdentityProvider>
// web components must be mounted on a page kbdb itself serves, so MCP
// clients can authenticate against kbdb standalone, without depending on
// mykeebs-web (the SPA) being deployed or reachable.
package consent

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/http"
	"text/template"
)

//go:embed authorize.html
var authorizeHTML string

// authorizeTemplate is parsed once at package init - authorize.html is a
// fixed, embedded asset, not user input, so a parse failure here is a
// build-time bug, not a runtime condition to handle gracefully.
var authorizeTemplate = template.Must(template.New("authorize.html").Parse(authorizeHTML))

// templateData is the data authorize.html's {{.StytchPublicToken}} and
// {{.OIDCIssuerBaseURL}} placeholders render against. Both fields are
// pre-quoted as Go string literals (via %q) before reaching the template,
// so they render directly into the page's JS as valid, safely-escaped
// string literals - text/template has no JS-string-context awareness the
// way html/template has for HTML/URL/JS contexts, so quoting is done by
// the caller, not left to the template engine.
type templateData struct {
	StytchPublicToken string
	OIDCIssuerBaseURL string
}

// Handler serves GET /authorize: kbdb's Stytch consent/login page.
// oidcIssuerBaseURL is passed to the Stytch SDK as customBaseUrl (see
// authorize.html) so Stytch stamps this environment's real issuer into the
// RFC 9207 "iss" it returns, matching what discovery already reports.
func Handler(stytchPublicToken, oidcIssuerBaseURL string) http.Handler {
	var buf bytes.Buffer
	data := templateData{
		StytchPublicToken: fmt.Sprintf("%q", stytchPublicToken),
		OIDCIssuerBaseURL: fmt.Sprintf("%q", oidcIssuerBaseURL),
	}
	if err := authorizeTemplate.Execute(&buf, data); err != nil {
		// authorize.html and templateData are both fixed at build time;
		// Execute can only fail here from a programmer error (e.g. a typo'd
		// field name), which should fail loudly rather than serve a broken
		// page silently.
		panic(fmt.Sprintf("consent: rendering authorize.html: %v", err))
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
