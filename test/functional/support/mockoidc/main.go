// Command mockoidc runs a standalone OIDC server for local dev and
// functional tests, standing in for Cognito. Unlike mockoidc.Run() (which
// binds an OS-assigned ephemeral port), this binds a fixed, known port so
// sam local start-api's env-vars file and the Ginkgo functional specs can
// both reference the same stable address.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

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

	if err := m.Start(ln, nil); err != nil {
		return fmt.Errorf("starting mockoidc server: %w", err)
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

	<-ctx.Done()
	return nil
}
