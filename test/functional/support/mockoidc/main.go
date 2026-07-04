// Command mockoidc runs a standalone OIDC server for local dev and
// functional tests, standing in for Cognito. Unlike mockoidc.Run() (which
// binds an OS-assigned ephemeral port), this binds a fixed, known port so
// sam local start-api's env-vars file and the Ginkgo functional specs can
// both reference the same stable address.
package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/oauth2-proxy/mockoidc"

	"github.com/rogueserenity/kbdb/test/functional/support/mockoidc/fixtures"
)

func main() {
	m, err := mockoidc.NewServer(nil)
	if err != nil {
		log.Fatalf("creating mockoidc server: %v", err)
	}
	m.ClientID = fixtures.TestClientID
	m.ClientSecret = fixtures.TestClientSecret

	ln, err := net.Listen("tcp", ":9999")
	if err != nil {
		log.Fatalf("binding listener: %v", err)
	}

	if err := m.Start(ln, nil); err != nil {
		log.Fatalf("starting mockoidc server: %v", err)
	}
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

	m.QueueUser(&mockoidc.MockUser{
		Subject:       fixtures.TestUserSubject,
		Email:         "test-user@rogueserenity.dev",
		EmailVerified: true,
	})

	cfg := m.Config()
	log.Printf("mockoidc listening on %s", m.Addr())
	log.Printf("issuer=%s client_id=%s client_secret=%s", cfg.Issuer, cfg.ClientID, cfg.ClientSecret)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
