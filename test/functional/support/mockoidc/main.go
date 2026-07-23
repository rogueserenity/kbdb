// Command mockoidc runs a standalone OIDC server for local dev and
// functional tests, standing in for Cognito. Unlike mockoidc.Run() (which
// binds an OS-assigned ephemeral port), this binds a fixed, known port so
// sam local start-api's env-vars file and the Ginkgo functional specs can
// both reference the same stable address.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oauth2-proxy/mockoidc"

	"github.com/rogueserenity/kbdb/test/functional/support/mockoidc/fixtures"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	m, err := mockoidc.NewServer(nil)
	if err != nil {
		return fmt.Errorf("creating mockoidc server: %w", err)
	}
	m.ClientID = fixtures.TestClientID
	m.ClientSecret = fixtures.TestClientSecret

	// Must bind all interfaces to be reachable from other containers on the
	// compose network; local-only test fixture, not a real security risk.
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", ":9999") //nolint:gosec
	if err != nil {
		return fmt.Errorf("binding listener: %w", err)
	}

	// mockoidc's Authorize/Token/Userinfo/JWKS/Discovery are exported
	// methods, so we wire them into our own mux alongside /test/queue-user.
	mux := http.NewServeMux()
	mux.HandleFunc(mockoidc.AuthorizationEndpoint, m.Authorize)
	mux.HandleFunc(mockoidc.TokenEndpoint, m.Token)
	mux.HandleFunc(mockoidc.UserinfoEndpoint, m.Userinfo)
	mux.HandleFunc(mockoidc.JWKSEndpoint, m.JWKS)
	mux.HandleFunc(mockoidc.DiscoveryEndpoint, m.Discovery)
	mux.HandleFunc("POST /test/queue-user", queueUserHandler(m))

	m.Server = &http.Server{
		Addr:              ln.Addr().String(),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := m.Server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	defer func() { _ = m.Shutdown() }()

	// m.Server.Addr defaults to the listener's bind address (e.g. "[::]:9999"
	// or "0.0.0.0:9999"), which isn't a usable hostname for any client outside
	// this process's own network namespace. It's baked into every signed
	// JWT's iss claim and the discovery document's issuer field, so it must
	// be overwritten with the address other containers/processes actually
	// use to reach this one (the docker-compose service name, or localhost
	// for a bare host run) before anything calls Issuer()/Discovery().
	advertiseAddr := os.Getenv("MOCKOIDC_ADVERTISE_ADDR")
	if advertiseAddr == "" {
		advertiseAddr = "localhost:9999"
	}
	m.Server.Addr = advertiseAddr

	cfg := m.Config()
	log.Printf("mockoidc listening on %s", m.Addr())
	log.Printf("issuer=%s client_id=%s client_secret=%s", cfg.Issuer, cfg.ClientID, cfg.ClientSecret)

	<-ctx.Done()
	return nil
}

// groups is optional - omitted/empty means no cognito:groups claim, i.e. a
// non-admin token.
type queueUserRequest struct {
	Subject string   `json:"subject"`
	Groups  []string `json:"groups"`
}

// queueUserHandler lets functional-test specs (a separate process from
// this server) pick the next minted token's identity - mockoidc.UserQueue
// is a strict FIFO with no selection mechanism otherwise.
func queueUserHandler(m *mockoidc.MockOIDC) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req queueUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.Subject == "" {
			http.Error(w, "subject is required", http.StatusBadRequest)
			return
		}

		base := &mockoidc.MockUser{
			Subject:       req.Subject,
			Email:         req.Subject + "@rogueserenity.dev",
			EmailVerified: true,
		}

		if len(req.Groups) == 0 {
			m.QueueUser(base)
		} else {
			m.QueueUser(&groupedUser{MockUser: base, Groups: req.Groups})
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// groupedUser wraps mockoidc.MockUser to add the cognito:groups claim by
// implementing Claims, the only extension point mockoidc's User interface
// offers for arbitrary claims.
type groupedUser struct {
	*mockoidc.MockUser
	Groups []string
}

func (u *groupedUser) Claims(scope []string, base *mockoidc.IDTokenClaims) (jwt.Claims, error) {
	inner, err := u.MockUser.Claims(scope, base)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(inner)
	if err != nil {
		return nil, err
	}
	var claims jwt.MapClaims
	if err := json.Unmarshal(b, &claims); err != nil {
		return nil, err
	}
	claims["cognito:groups"] = u.Groups
	return claims, nil
}
