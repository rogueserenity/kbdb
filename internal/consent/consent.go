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

// templateData is the data authorize.html's {{.StytchPublicToken}}
// placeholder renders against. StytchPublicToken is pre-quoted as a Go
// string literal (via %q) before reaching the template, so it renders
// directly into the page's JS as a valid, safely-escaped string literal -
// text/template has no JS-string-context awareness the way html/template
// has for HTML/URL/JS contexts, so quoting is done by the caller, not left
// to the template engine.
type templateData struct {
	StytchPublicToken string
}

// Handler serves GET /authorize: kbdb's Stytch consent/login page.
// stytchPublicToken is the environment's Stytch public token (safe to
// embed client-side by design - it authorizes the client-side SDK, not a
// privileged operation), rendered into the page at handler-construction
// time since it varies per dev/CI/prod stack.
func Handler(stytchPublicToken string) http.Handler {
	var buf bytes.Buffer
	data := templateData{StytchPublicToken: fmt.Sprintf("%q", stytchPublicToken)}
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
