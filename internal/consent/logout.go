package consent

import (
	"bytes"
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"text/template"

	"github.com/rogueserenity/kbdb/internal/problem"
)

//go:embed logout.html
var logoutHTML string

var logoutTemplate = template.Must(template.New("logout.html").Parse(logoutHTML))

type logoutTemplateData struct {
	StytchPublicToken string
	OIDCIssuerBaseURL string
	ReturnTo          string
}

// LogoutHandler serves GET /logout: revokes the Stytch session on this
// origin (see internal/consent's package doc for why that can't be done
// from mykeebs-web's own origin), then redirects to return_to.
// allowedReturnOrigins restricts return_to to known callers, since it would
// otherwise be an open redirect.
func LogoutHandler(stytchPublicToken, oidcIssuerBaseURL string, allowedReturnOrigins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		returnTo, err := validReturnTo(r.URL.Query().Get("return_to"), allowedReturnOrigins)
		if err != nil {
			problem.BadRequest(w, err.Error())
			return
		}

		data := logoutTemplateData{
			StytchPublicToken: fmt.Sprintf("%q", stytchPublicToken),
			OIDCIssuerBaseURL: fmt.Sprintf("%q", oidcIssuerBaseURL),
			ReturnTo:          fmt.Sprintf("%q", returnTo),
		}

		var buf bytes.Buffer
		if err := logoutTemplate.Execute(&buf, data); err != nil {
			log.Fatalf("consent: rendering logout.html: %v", err)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(buf.Bytes())
	})
}

// validReturnTo rejects a return_to whose origin isn't in allowedOrigins.
func validReturnTo(returnTo string, allowedOrigins []string) (string, error) {
	if returnTo == "" {
		return "", fmt.Errorf("return_to is required")
	}

	parsed, err := url.Parse(returnTo)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("return_to must be an absolute URL")
	}

	origin := parsed.Scheme + "://" + parsed.Host
	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return returnTo, nil
		}
	}

	return "", fmt.Errorf("return_to origin %q is not allowed", origin)
}
